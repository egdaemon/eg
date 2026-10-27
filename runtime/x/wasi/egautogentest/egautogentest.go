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
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
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
}

// DefaultRevisions is the number of rework rounds used when none are specified.
const DefaultRevisions = 3

type option struct {
	model     string
	attempts  int // number of generations to try per function, defaults to 1.
	ensure    []string
	revisions uint
	client    egllm.Client
}

// Option configures test generation.
type Option func(*option)

// Options is a chainable set of Option.
type Options []Option

// New generates the default options.
func New() Options {
	return Options(nil)
}

// Model sets the model used to generate the tests.
func (t Options) Model(v string) Options {
	return append(t, func(o *option) {
		o.model = v
	})
}

// Attempts sets the number of generations to try per function.
func (t Options) Attempts(n int) Options {
	return append(t, func(o *option) {
		o.attempts = n
	})
}

// Ensure adds rules every generated test must satisfy, e.g.) "no helper functions are added".
// a test violating the rules is sent back to the model to be reworked.
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
	o := option{revisions: DefaultRevisions, client: egllm.New()}
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

	return egllm.With(t.model, op)
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
	log.Printf("autogentest %s: prompt (%d bytes)\n%s\n", fn.Name, len(prompt), prompt)

	attempts := max(t.attempts, 1)
	for attempt := 1; attempt <= attempts; attempt++ {
		astart := time.Now()
		log.Printf("autogentest %s attempt %d/%d started\n", fn.Name, attempt, attempts)
		result, err := egllm.Generate(ctx, t.client, t.model, prompt)
		if err != nil {
			return false, err
		}
		log.Printf("autogentest %s attempt %d/%d: model response (%d bytes) in %v\n%s\n", fn.Name, attempt, attempts, len(result), time.Since(astart), result)

		if len(t.ensure) > 0 {
			var ok bool
			if result, ok, err = t.rework(ctx, prompt, result); err != nil {
				return false, err
			} else if !ok {
				log.Printf("autogentest %s attempt %d/%d: skipped, result violating rules\n", fn.Name, attempt, attempts)
				continue
			}
			log.Printf("autogentest %s attempt %d/%d: rules satisfied, final result\n%s\n", fn.Name, attempt, attempts, result)
		}

		if err := g.Verify(ctx, fn, result); err != nil {
			log.Printf("autogentest %s attempt %d/%d: skipped, failing result after %v: %v\n", fn.Name, attempt, attempts, time.Since(astart), err)
			continue
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
if the response satisfies every rule respond with nothing at all.
you must not respond with prose, code blocks, or an array.

--------------------------------------------------- RULES ---------------------------------------------------
:rules:
--------------------------------------------------- RESPONSE ---------------------------------------------------
:response:
`

const reviseTemplate = `
your response violates the following rules:
:violations:

rewrite your response so it satisfies every rule while still accomplishing the original request.
respond with only the complete file, as plain text without code fences or any other text.
`

// violation is a single place a response breaks one of the rules.
type violation struct {
	Rule   int    `json:"rule"` // 1 based index into the rules.
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// parseViolations decodes the judge's JSONL response, one violation per record.
func parseViolations(response string, rules int) (violations []violation, err error) {
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

// judge asks the model which of the rules the response violates.
func (t option) judge(ctx context.Context, response string) ([]violation, error) {
	var rules strings.Builder
	for i, rule := range t.ensure {
		fmt.Fprintf(&rules, "%d. %s\n", i+1, rule)
	}

	var numbered strings.Builder
	for i, line := range strings.Split(response, "\n") {
		fmt.Fprintf(&numbered, "%d: %s\n", i+1, line)
	}

	prompt := strings.NewReplacer(
		":rules:", rules.String(),
		":response:", numbered.String(),
	).Replace(judgeTemplate)

	judged, err := egllm.Generate(ctx, t.client, t.model, prompt)
	if err != nil {
		return nil, err
	}

	violations, err := parseViolations(judged, len(t.ensure))
	if err != nil {
		log.Printf("autogentest unable to parse judge response:\n%s\n", judged)
	}

	return violations, err
}

// rework checks the generated response against the rules. violations are sent back to the model,
// with the source material and the response it generated as context, to be reworked until the rules
// hold or the revisions are exhausted. returns false if the rules still do not hold.
func (t option) rework(ctx context.Context, source string, generated string) (string, bool, error) {
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

		if round == t.revisions {
			break
		}

		var described strings.Builder
		for _, v := range violations {
			fmt.Fprintf(&described, "- rule %d (%s) line %d: %s\n", v.Rule, t.ensure[v.Rule-1], v.Line, v.Reason)
		}

		if generated, err = egllm.Chat(
			ctx,
			t.client,
			t.model,
			egllm.Message{Role: "user", Content: source},
			egllm.Message{Role: "assistant", Content: generated},
			egllm.Message{Role: "user", Content: strings.NewReplacer(":violations:", described.String()).Replace(reviseTemplate)},
		); err != nil {
			return "", false, err
		}
		log.Printf("autogentest round %d/%d: violations\n%srevised response\n%s\n", round, t.revisions, described.String(), generated)
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

// Worst scans recorded coverage for the n functions with the lowest hit counts.
func Worst(n int) iterx.Seq[Fn] {
	return iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		results, err := fficoverage.Worst(ctx, int32(n))
		if err != nil {
			return err
		}

		for _, c := range results {
			if !yield(Fn{Path: c.Path, Name: c.Fnname, Hits: int(c.Hits)}) {
				return nil
			}
		}

		return nil
	})
}

// From builds a Seq directly from a fixed set of functions, useful for
// driving a Generator without recorded coverage data (e.g. in tests or
// one-off manual runs).
func From(fns ...Fn) iterx.Seq[Fn] {
	return iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		for _, fn := range fns {
			if !yield(fn) {
				return nil
			}
		}

		return nil
	})
}

// Sample returns a random sample of n functions from recorded coverage data.
func Sample(n int) iterx.Seq[Fn] {
	return iterx.New(func(ctx context.Context, yield func(Fn) bool) error {
		results, err := fficoverage.Sample(ctx, int32(n))
		if err != nil {
			return err
		}

		for _, c := range results {
			if !yield(Fn{Path: c.Path, Name: c.Fnname, Hits: int(c.Hits)}) {
				return nil
			}
		}

		return nil
	})
}
