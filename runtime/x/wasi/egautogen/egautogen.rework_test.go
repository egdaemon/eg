package egautogen

import (
	"errors"
	"strings"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestRework(t *testing.T) {
	ensure := []string{"no helper functions are added"}
	same := func(string, string) (string, error) { return "prompt", nil }
	pass := func(string) error { return nil }

	t.Run("keeps a response satisfying the rules without reworking it", func(t *testing.T) {
		s := egllm.Constant("")
		o := option{ensure: ensure, revisions: 2, llm: s}

		got, ok, err := o.rework(t.Context(), same, pass, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "generated", got)
		require.Len(t, s.Requests(), 1)
	})

	t.Run("revises with the prompt rendered from the response and its violations", func(t *testing.T) {
		v := violationrecord(1, 5, "helper is declared")
		s := egllm.Constant(v, "reworked", v, "reworked again", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		var previous, violations []string
		reprompt := func(p string, v string) (string, error) {
			previous = append(previous, p)
			violations = append(violations, v)
			return "prompt for " + p, nil
		}

		got, ok, err := o.rework(t.Context(), reprompt, pass, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "reworked again", got)
		require.Equal(t, []string{"generated", "reworked"}, previous)
		for _, v := range violations {
			require.Contains(t, v, "rule 1 (no helper functions are added) line 5: helper is declared")
		}

		require.Len(t, s.Requests(), 5)
		require.Equal(t, []egllm.Message{{Role: "user", Content: "prompt for generated"}}, s.Requests()[1])
		require.Equal(t, []egllm.Message{{Role: "user", Content: "prompt for reworked"}}, s.Requests()[3])
		require.Contains(t, s.Requests()[2][0].Content, "reworked")
	})

	t.Run("fails once the revisions are exhausted", func(t *testing.T) {
		v := violationrecord(1, 5, "helper is declared")
		s := egllm.Constant(v, "reworked", v, "reworked again", v)
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), same, pass, "generated")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.Requests(), 5)
	})

	t.Run("zero revisions fails without reworking", func(t *testing.T) {
		s := egllm.Constant(violationrecord(1, 5, "helper is declared"))
		o := option{ensure: ensure, llm: s}

		_, ok, err := o.rework(t.Context(), same, pass, "generated")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.Requests(), 1)
	})

	t.Run("an undecodable judgement uses a round", func(t *testing.T) {
		s := egllm.Constant("looks fine to me", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), same, pass, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Len(t, s.Requests(), 2)
	})

	t.Run("numbers the rules and the response for the judge", func(t *testing.T) {
		s := egllm.Constant("")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, _, err := o.rework(t.Context(), same, pass, "first\nsecond")
		require.NoError(t, err)

		prompt := s.Requests()[0][0].Content
		require.True(t, strings.Contains(prompt, "1. no helper functions are added"))
		require.True(t, strings.Contains(prompt, "1: first\n2: second"))
	})

	t.Run("verifies each revision before judging it", func(t *testing.T) {
		s := egllm.Constant(violationrecord(1, 1, "dirty"), "", "")
		var verified []string
		o := option{ensure: ensure, revisions: 2, llm: s}

		got, ok, err := o.rework(t.Context(), same, func(v string) error {
			verified = append(verified, v)
			require.Len(t, s.Requests(), 2, "verified before the revision was judged")
			return nil
		}, "dirty")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []string{got}, verified)
	})

	t.Run("a revision failing verification fails without judging it", func(t *testing.T) {
		s := egllm.Constant(violationrecord(1, 1, "dirty"), "revised")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), same, func(string) error { return errors.New("failing") }, "dirty")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.Requests(), 2)
	})
}
