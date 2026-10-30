package egautogen

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

type fakegen struct {
	seq      iterx.Seq[Fn]
	ensured  []string
	verified []string
	previous []string
	results  []string
	violated []string
	verifyfn func(artifact string) error
}

func (t *fakegen) Seq() iterx.Seq[Fn] {
	return t.seq
}

func (t *fakegen) Prompt(c PromptContext) (string, error) {
	t.ensured = c.Ensure
	t.previous = append(t.previous, c.Previous)
	t.results = append(t.results, c.Results)
	t.violated = append(t.violated, c.Violations)
	return "prompt for " + c.Fn.Name, nil
}

func (t *fakegen) Verify(ctx context.Context, fn Fn, artifact string) error {
	t.verified = append(t.verified, artifact)
	if t.verifyfn != nil {
		return t.verifyfn(artifact)
	}

	return nil
}

type rulergen struct {
	fakegen
	rules []string
}

func (t *rulergen) Rules(ensure ...string) []string {
	return append(append([]string(nil), t.rules...), ensure...)
}

func TestRunAllFunctions(t *testing.T) {
	fns := []Fn{{Name: "One"}, {Name: "Two"}, {Name: "Three"}}

	t.Run("generates for every function", func(t *testing.T) {
		s := egllm.Constant("a", "b", "c")
		g := &fakegen{seq: FromSpecified(fns...)}
		o := New().Judge(s)

		require.NoError(t, o.Run(g)(t.Context(), nil))
		require.Len(t, g.verified, 3)
	})
}

func TestGenerate(t *testing.T) {
	fn := Fn{Name: "Greet"}

	t.Run("verifies the response as is", func(t *testing.T) {
		s := egllm.Constant("package sample\n")
		g := &fakegen{}
		o := option{llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"package sample\n"}, g.verified)
		require.Equal(t, "prompt for Greet", s.Requests()[0][0].Content)
	})

	t.Run("gives the rules to the prompt", func(t *testing.T) {
		s := egllm.Constant("response", "")
		g := &fakegen{}
		o := option{ensure: []string{"a", "b"}, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"a", "b"}, g.ensured)
	})

	t.Run("does not judge without rules", func(t *testing.T) {
		s := egllm.Constant("response")
		o := option{llm: s}

		require.NoError(t, o.generate(t.Context(), &fakegen{}, fn))
		require.Len(t, s.Requests(), 1)
	})

	t.Run("retries when verification fails, up to the attempts", func(t *testing.T) {
		s := egllm.Constant("one", "two", "three")
		g := &fakegen{verifyfn: func(string) error { return errors.New("failing") }}
		o := option{attempts: 3, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Len(t, g.verified, 3)
		require.Len(t, s.Requests(), 3)
	})

	t.Run("gives only the previous response to the prompt of the retry", func(t *testing.T) {
		s := egllm.Constant("one", "two", "three")
		g := &fakegen{verifyfn: func(string) error { return errors.New("failing") }}
		o := option{attempts: 3, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"", "one", "two"}, g.previous)
	})

	t.Run("stops retrying once verification passes", func(t *testing.T) {
		s := egllm.Constant("one", "two")
		failed := false
		g := &fakegen{verifyfn: func(string) error {
			if !failed {
				failed = true
				return errors.New("failing")
			}
			return nil
		}}
		o := option{attempts: 3, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Len(t, g.verified, 2)
		require.Len(t, s.Requests(), 2)
	})

	t.Run("verifies the reworked response", func(t *testing.T) {
		s := egllm.Constant("dirty", violationrecord(1, 1, "dirty"), "clean", "")
		g := &fakegen{}
		o := option{ensure: []string{"be clean"}, revisions: 1, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"dirty", "clean"}, g.verified)
		require.Equal(t, []string{"", "dirty"}, g.previous)
	})

	t.Run("judges against the rules of the generator as well as the given rules", func(t *testing.T) {
		s := egllm.Constant("response", "")
		g := &rulergen{rules: []string{"keep the header"}}
		o := option{ensure: []string{"be clean"}, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Len(t, s.Requests(), 2)
		require.Contains(t, s.Requests()[1][0].Content, "1. keep the header\n2. be clean\n")
	})

	t.Run("does not judge when only the generator has rules", func(t *testing.T) {
		s := egllm.Constant("response")
		g := &rulergen{rules: []string{"keep the header"}}
		o := option{llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Len(t, s.Requests(), 1)
	})

	t.Run("verifies before checking the rules, then verifies again after reworking", func(t *testing.T) {
		s := egllm.Constant("dirty", violationrecord(1, 1, "dirty"), "clean", "")
		var order []string
		g := &fakegen{verifyfn: func(a string) error {
			order = append(order, fmt.Sprintf("verify %s after %d requests", a, len(s.Requests())))
			return nil
		}}
		o := option{ensure: []string{"be clean"}, revisions: 1, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"verify dirty after 1 requests", "verify clean after 3 requests"}, order)
	})

	t.Run("does not check the rules of a response failing verification", func(t *testing.T) {
		s := egllm.Constant("broken")
		g := &fakegen{verifyfn: func(string) error { return errors.New("failing") }}
		o := option{ensure: []string{"be clean"}, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"broken"}, g.verified)
		require.Len(t, s.Requests(), 1)
	})

	t.Run("discards a revision failing verification and tries again", func(t *testing.T) {
		s := egllm.Constant("dirty", violationrecord(1, 1, "dirty"), "broken", "clean", "")
		g := &fakegen{verifyfn: func(a string) error {
			if a == "broken" {
				return errors.New("failing")
			}
			return nil
		}}
		o := option{attempts: 2, ensure: []string{"be clean"}, revisions: 1, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"dirty", "broken", "clean"}, g.verified)
	})

	t.Run("a response still violating the rules was verified", func(t *testing.T) {
		s := egllm.Constant("dirty", violationrecord(1, 1, "dirty"))
		g := &fakegen{}
		o := option{ensure: []string{"be clean"}, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"dirty"}, g.verified)
	})

	t.Run("gives why the response failed verification to the prompt of the retry", func(t *testing.T) {
		s := egllm.Constant("one", "two")
		g := &fakegen{verifyfn: func(a string) error {
			if a == "one" {
				return errors.New("does not compile")
			}
			return nil
		}}
		o := option{attempts: 2, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"", "one"}, g.previous)
		require.Equal(t, []string{"", "does not compile"}, g.results)
	})

	t.Run("gives the violations to the prompt of the revision", func(t *testing.T) {
		s := egllm.Constant("dirty", violationrecord(1, 1, "dirty"), "clean", "")
		g := &fakegen{}
		o := option{ensure: []string{"be clean"}, revisions: 1, llm: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"", "dirty"}, g.previous)
		require.Equal(t, "", g.violated[0])
		require.Contains(t, g.violated[1], "rule 1 (be clean) line 1: dirty")
		require.Equal(t, "prompt for Greet", s.Requests()[2][0].Content)
	})
}
