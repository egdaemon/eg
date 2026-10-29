// Package egautogentest drives language-specific test generators against
// functions selected from the coverage data eg already records while
// running a module's tests.
package egautogentest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
	"github.com/egdaemon/eg/runtime/x/wasi/egdb"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

// Fn identifies a single function discovered from recorded coverage data.
type Fn struct {
	Path string
	Name string
	Hits int
}

// Generator implements the language specific parts of test generation.
// it prepares the material the model works from and verifies the model's response,
// which is expected to be only the artifact.
type Generator interface {
	// Seq are the functions to generate tests for.
	Seq() iterx.Seq[Fn]
	// Prompt is the source material the model works from, typically rendered
	// from a text/template executed with the context.
	Prompt(c PromptContext) (string, error)
	// Verify persists and validates the artifact, the model's response, which the prompt
	// requires to be only the artifact. an error discards the artifact and must leave nothing behind.
	Verify(ctx context.Context, fn Fn, artifact string) error
}

// PromptContext is the data a Generator renders its prompt from.
type PromptContext struct {
	// Fn is the function to generate a test for.
	Fn Fn
	// Ensure are the rules the result must satisfy.
	Ensure []string
	// Previous is the response of the attempt before this one for Fn, empty on the first attempt.
	Previous string
	// Violations describes the rules Previous violates, empty when it satisfied them.
	Violations string
	// Results is why Previous failed verification, e.g.) the output of the compiler and the tests, empty when it passed.
	Results string
}

// DefaultRevisions is the number of rework rounds used when none are specified.
const DefaultRevisions = 3

type option struct {
	attempts  int // number of generations to try per function, defaults to 1.
	ensure    []string
	revisions uint
	llm       egllm.Judge
}

// Option configures test generation.
type Option func(*option)

// Ruler is implemented by generators that apply rules of their own in addition to the ones given, e.g.)
// defaults. responses are judged against the rules it returns, so what the judge considers a violation
// matches what the prompt asked for.
type Ruler interface {
	// Rules are all the rules the generator applies when it is given ensure.
	Rules(ensure ...string) []string
}

// Options is a chainable set of Option.
type Options []Option

// New generates the default options.
func New() Options {
	return Options(nil)
}

// Judge sets the judge used to generate the tests, defaults to egllm.Echo.
func (t Options) Judge(v egllm.Judge) Options {
	return append(t, func(o *option) {
		o.llm = v
	})
}

// Attempts sets the number of generations to try per function.
func (t Options) Attempts(n int) Options {
	return append(t, func(o *option) {
		o.attempts = n
	})
}

// Ensure adds rules every generated test must satisfy, e.g.) "no helper functions are added".
// rules are checked once the test is verified, a test violating them is sent back to the model to be reworked
// and the revision is verified again.
func (t Options) Ensure(rules ...string) Options {
	return append(t, func(o *option) {
		o.ensure = append(o.ensure, rules...)
	})
}

// Revisions sets the maximum number of times a test violating the rules is sent back
// to the model to be reworked. 0 discards a violating test without reworking it.
func (t Options) Revisions(n uint) Options {
	return append(t, func(o *option) {
		o.revisions = n
	})
}

// Run generates tests with g subject to the options.
func (t Options) Run(g Generator) eg.OpFn {
	o := option{revisions: DefaultRevisions, llm: egllm.Echo()}
	for _, opt := range t {
		opt(&o)
	}

	return o.run(g)
}

func (t option) run(g Generator) eg.OpFn {
	seq := g.Seq()

	op := func(ctx context.Context, _ eg.Op) error {
		var passed, failed uint
		started := time.Now()
		for fn := range seq.Each(ctx) {
			log.Printf("autogentest [%d] %s:%s hits=%d started\n", passed+failed+1, fn.Path, fn.Name, fn.Hits)
			fstart := time.Now()
			ok, err := t.try(ctx, g, fn)
			if err != nil {
				log.Printf("autogentest %s:%s errored after %v: %v\n", fn.Path, fn.Name, time.Since(fstart), err)
				return err
			}

			if ok {
				passed++
			} else {
				failed++
			}
			log.Printf("autogentest %s:%s completed ok=%t in %v\n", fn.Path, fn.Name, ok, time.Since(fstart))
		}

		log.Printf("autogentest summary: selected=%d passed=%d failed=%d elapsed=%v\n", passed+failed, passed, failed, time.Since(started))
		return seq.Err()
	}

	if s, ok := t.llm.(egllm.Server); ok {
		return s.With(op)
	}

	return op
}

func (t option) generate(ctx context.Context, g Generator, fn Fn) error {
	_, err := t.try(ctx, g, fn)
	return err
}

// try reports whether a verified result was produced for fn.
func (t option) try(ctx context.Context, g Generator, fn Fn) (bool, error) {
	prompt, err := g.Prompt(PromptContext{Fn: fn, Ensure: t.ensure})
	if err != nil {
		return false, err
	}
	debugx.Fn(func() { log.Printf("autogentest %s: prompt (%d bytes)\n%s\n", fn.Name, len(prompt), prompt) })

	// the judge needs to know every rule the prompt asked for, not only the ones given.
	judge := t
	if r, ok := g.(Ruler); ok {
		judge.ensure = r.Rules(t.ensure...)
	}

	reprompt := func(previous string, violations string) (string, error) {
		return g.Prompt(PromptContext{Fn: fn, Ensure: t.ensure, Previous: previous, Violations: violations})
	}

	// the last response and why it failed verification, for the prompt of the next attempt to learn from.
	var previous, results string
	verify := func(artifact string) error {
		previous, results = artifact, ""
		err := g.Verify(ctx, fn, artifact)
		if err != nil {
			results = err.Error()
		}

		return err
	}

	attempts := max(t.attempts, 1)
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			// let the generator learn from the previous attempt.
			if prompt, err = g.Prompt(PromptContext{Fn: fn, Ensure: t.ensure, Previous: previous, Results: results}); err != nil {
				return false, err
			}
			debugx.Fn(func() {
				log.Printf("autogentest %s attempt %d/%d: prompt (%d bytes)\n%s\n", fn.Name, attempt, attempts, len(prompt), prompt)
			})
		}

		astart := time.Now()
		log.Printf("autogentest %s attempt %d/%d started\n", fn.Name, attempt, attempts)
		result, err := t.llm.Do(ctx, egllm.Message{Role: "user", Content: prompt})
		if err != nil {
			return false, err
		}
		log.Printf("autogentest %s attempt %d/%d: model response (%d bytes) in %v\n", fn.Name, attempt, attempts, len(result), time.Since(astart))
		debugx.Fn(func() {
			log.Printf("autogentest %s attempt %d/%d: model response\n%s\n", fn.Name, attempt, attempts, result)
		})

		if err := verify(result); err != nil {
			log.Printf("autogentest %s attempt %d/%d: skipped, failing result after %v: %v\n", fn.Name, attempt, attempts, time.Since(astart), err)
			continue
		}

		if len(t.ensure) > 0 {
			var ok bool
			if result, ok, err = judge.rework(ctx, reprompt, verify, result); err != nil {
				return false, err
			} else if !ok {
				log.Printf("autogentest %s attempt %d/%d: skipped, result violating rules or failing after rework\n", fn.Name, attempt, attempts)
				continue
			}
			debugx.Fn(func() {
				log.Printf("autogentest %s attempt %d/%d: rules satisfied, final result\n%s\n", fn.Name, attempt, attempts, result)
			})
		}

		log.Printf("autogentest %s attempt %d/%d completed, passing result in %v\n", fn.Name, attempt, attempts, time.Since(astart))
		return true, nil
	}

	log.Printf("autogentest %s:%s unable to generate a passing result after %d attempts\n", fn.Path, fn.Name, attempts)
	return false, nil
}

const judgeTemplate = `
you are reviewing a response against a numbered list of rules.
for every place the response violates a rule respond with one JSON object on its own line (JSONL) and nothing else:
{"rule": <rule number>, "line": <line number within the response>, "reason": "<why the line violates the rule>"}
if the response satisfies every rule respond with nothing at all, an empty message, not "[]", "none", or any other text.
you must not respond with prose, code blocks, or an array.

--------------------------------------------------- RULES ---------------------------------------------------
:rules:
--------------------------------------------------- RESPONSE ---------------------------------------------------
:response:
`

// violation is a single place a response breaks one of the rules.
type violation struct {
	Rule   int    `json:"rule"` // 1 based index into the rules.
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// parseViolations decodes the judge's JSONL response, one violation per record.
// an empty array is accepted as no violations, models often answer that way when there are none.
func parseViolations(response string, rules int) (violations []violation, err error) {
	if strings.TrimSpace(response) == "[]" {
		return nil, nil
	}

	dec := json.NewDecoder(strings.NewReader(response))
	for {
		var v violation
		if err := dec.Decode(&v); err == io.EOF {
			return violations, nil
		} else if err != nil {
			return nil, errorsx.Wrap(err, "unable to decode violation")
		}

		if v.Rule < 1 || v.Rule > rules {
			return nil, fmt.Errorf("violation references unknown rule: %d", v.Rule)
		}

		violations = append(violations, v)
	}
}

// numberLines prefixes each line of the text with its number, the numbers violations refer to.
func numberLines(text string) string {
	var numbered strings.Builder
	for i, line := range strings.Split(text, "\n") {
		fmt.Fprintf(&numbered, "%d: %s\n", i+1, line)
	}

	return numbered.String()
}

// judge asks the model which of the rules the response violates.
func (t option) judge(ctx context.Context, response string) ([]violation, error) {
	var rules strings.Builder
	for i, rule := range t.ensure {
		fmt.Fprintf(&rules, "%d. %s\n", i+1, rule)
	}

	prompt := strings.NewReplacer(
		":rules:", rules.String(),
		":response:", numberLines(response),
	).Replace(judgeTemplate)

	judged, err := t.llm.Do(ctx, egllm.Message{Role: "user", Content: prompt})
	if err != nil {
		return nil, err
	}

	violations, err := parseViolations(judged, len(t.ensure))
	if err != nil {
		log.Printf("autogentest unable to parse judge response:\n%s\n", judged)
	}

	return violations, err
}

// rework checks the verified response against the rules. violations are sent back to the model, in the
// prompt reprompt renders from the response and the violations, to be reworked until the rules
// hold or the revisions are exhausted. returns false if the rules still do not hold.
// each revision is verified before the rules are checked again, one failing verification returns false.
// a revision can use apis the prompt never showed the model, which rendering it again from the revision includes.
func (t option) rework(ctx context.Context, reprompt func(previous string, violations string) (string, error), verify func(generated string) error, generated string) (string, bool, error) {
	for round := uint(0); round <= t.revisions; round++ {
		violations, err := t.judge(ctx, generated)
		if err != nil {
			log.Printf("autogentest round %d: unable to judge response: %v\n", round, err)
			continue
		}

		log.Printf("autogentest round %d/%d: %d violations\n", round, t.revisions, len(violations))
		if len(violations) == 0 {
			return generated, true, nil
		}

		var described strings.Builder
		for _, v := range violations {
			fmt.Fprintf(&described, "- rule %d (%s) line %d: %s\n", v.Rule, t.ensure[v.Rule-1], v.Line, v.Reason)
		}
		debugx.Fn(func() { log.Printf("autogentest round %d/%d: violations\n%s", round, t.revisions, described.String()) })

		if round == t.revisions {
			break
		}

		revise, err := reprompt(generated, described.String())
		if err != nil {
			return "", false, err
		}
		debugx.Fn(func() { log.Printf("autogentest round %d/%d: next prompt\n%s\n", round, t.revisions, revise) })

		if generated, err = t.llm.Do(ctx, egllm.Message{Role: "user", Content: revise}); err != nil {
			return "", false, err
		}
		debugx.Fn(func() { log.Printf("autogentest round %d/%d: revised response\n%s\n", round, t.revisions, generated) })

		if err = verify(generated); err != nil {
			log.Printf("autogentest round %d/%d: revision failed verification: %v\n", round, t.revisions, err)
			return "", false, nil
		}
	}

	return "", false, nil
}

// writeTemp writes data to a new file in dir named by pattern, see os.CreateTemp. the leading dot
// of the pattern makes go ignore it. it is within dir so that it can be renamed atomically into place.
func writeTemp(dir string, pattern string, data []byte) (path string, err error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to create temporary file")
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(f.Name()))
		}
	}()

	if _, err = f.Write(data); err != nil {
		return "", errors.Join(err, f.Close())
	}

	if err = f.Chmod(0644); err != nil {
		return "", errors.Join(err, f.Close())
	}

	if err = f.Close(); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// Language selects the functions of files in any of the languages, e.g.) "go", matched on the extension
// of the file. the name is not case sensitive and an unknown language is an error.
func Language(names ...string) coverage.Filter {
	return coverage.QueryLanguage(names...)
}

// Paths selects the functions of files matching any of the glob patterns. patterns match the whole recorded
// path, which is relative to the workspace. `*` matches any run of characters including separators, `?` any single character.
func Paths(globs ...string) coverage.Filter {
	return coverage.QueryPaths(globs...)
}

// ExcludePaths removes the functions of files matching any of the glob patterns, see Paths.
func ExcludePaths(globs ...string) coverage.Filter {
	return coverage.QueryExcludePaths(globs...)
}

// Coverage selects the functions whose statement coverage is within min and max percent, both inclusive.
func Coverage(min, max float32) coverage.Filter {
	return coverage.QueryCoverage(min, max)
}

// Worst is the query selecting the functions with the lowest statement coverage first, of those matching the filters.
// it is not run, see From, nor limited, see Seq.Limit.
func Worst(filters ...coverage.Filter) squirrel.SelectBuilder {
	return coverage.WorstSearchBuilder(filters...)
}

// Sample is the query selecting the functions matching the filters in a random order. it is not run, see From, nor
// limited, apply Seq.Limit for a sample of n functions.
func Sample(filters ...coverage.Filter) squirrel.SelectBuilder {
	return coverage.SampleSearchBuilder(filters...)
}

// FromDefault runs sb against the coverage recorded by the last successful run of the default workload, see FromWorkload.
func FromDefault(sb squirrel.SelectBuilder) Seq {
	return FromWorkload(_eg.DefaultWorkload, sb)
}

// FromWorkload runs sb against the coverage recorded by the last successful run of the workload, instead of the
// coverage recorded by this run, see FromRun. the run is looked up, see egdb.Latest, while the functions are selected,
// which fails when the workload has no successful run.
func FromWorkload(workload string, sb squirrel.SelectBuilder) Seq {
	return Seq{iterx.Lazy(func(ctx context.Context) (iterx.Seq[Fn], io.Closer, error) {
		path, err := egdb.Latest(ctx, egdb.WithWorkload(workload))
		if err != nil {
			return nil, nil, err
		}

		return FromPath(path, sb).Seq, nil, nil
	})}
}

// FromPath runs sb against the coverage recorded in the database file at path, e.g.) the file provided by egdb.Latest.
// the file is opened when iteration starts and closed when it ends, including when it stops early.
func FromPath(path string, sb squirrel.SelectBuilder) Seq {
	return Seq{iterx.Lazy(func(ctx context.Context) (iterx.Seq[Fn], io.Closer, error) {
		f, err := egdb.OpenFile(ctx, path)
		if err != nil {
			return nil, nil, err
		}

		return From(f, sb).Seq, f, nil
	})}
}

// From runs sb against q, e.g.) a database opened with egdb.Open. q belongs to the caller and is not closed.
func From(q sqlx.Queryer, sb squirrel.SelectBuilder) Seq {
	return functions(func(context.Context) iterx.Seq[coverage.Report] {
		return coverage.Query(q, sb)
	})
}

// FromRun runs sb against the coverage recorded by this run.
func FromRun(sb squirrel.SelectBuilder) Seq {
	return functions(func(context.Context) iterx.Seq[coverage.Report] {
		return fficoverage.From(sb)
	})
}

// functions builds a Seq from the coverage reports. paths within the working directory are made relative to it,
// coverage can report them absolute, which a Generator working in another directory, such as a worktree, cannot move into it.
func functions(query func(context.Context) iterx.Seq[coverage.Report]) Seq {
	return Seq{iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		reports := query(ctx)
		for c := range reports.Each(ctx) {
			if !yield(Fn{Path: relativeTo(egenv.WorkingDirectory(), c.Path), Name: c.Fnname, Hits: int(c.Hits)}) {
				return nil
			}
		}

		return reports.Err()
	})}
}

func relativeTo(dir string, p string) string {
	if !filepath.IsAbs(p) {
		return p
	}

	rel, err := filepath.Rel(dir, p)
	if err != nil || !filepath.IsLocal(rel) {
		return p
	}

	return rel
}

// FromSpecified builds a Seq directly from a fixed set of functions, useful for
// driving a Generator without recorded coverage data (e.g. in tests or
// one-off manual runs). paths are made relative to the working directory like recorded coverage.
func FromSpecified(fns ...Fn) Seq {
	return Seq{iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		for _, fn := range fns {
			fn.Path = relativeTo(egenv.WorkingDirectory(), fn.Path)
			if !yield(fn) {
				return nil
			}
		}

		return nil
	})}
}
