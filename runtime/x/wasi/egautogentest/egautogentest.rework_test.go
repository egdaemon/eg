package egautogentest

import (
	"strings"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestRework(t *testing.T) {
	ensure := []string{"no helper functions are added"}
	same := func(string) (string, error) { return "source", nil }

	t.Run("keeps a response satisfying the rules without reworking it", func(t *testing.T) {
		s := egllm.Constant("")
		o := option{ensure: ensure, revisions: 2, llm: s}

		got, ok, err := o.rework(t.Context(), "source", same, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "generated", got)
		require.Len(t, s.Requests(), 1)
	})

	t.Run("sends the source and the generated response back with the violations", func(t *testing.T) {
		s := egllm.Constant(violationrecord(1, 5, "helper is declared"), "reworked", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		got, ok, err := o.rework(t.Context(), "source", same, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "reworked", got)
		require.Len(t, s.Requests(), 3)

		revise := s.Requests()[1]
		require.Len(t, revise, 3)
		require.Equal(t, egllm.Message{Role: "user", Content: "source"}, revise[0])
		require.Equal(t, egllm.Message{Role: "assistant", Content: "generated"}, revise[1])
		require.Equal(t, "user", revise[2].Role)
		require.Contains(t, revise[2].Content, "rule 1 (no helper functions are added) line 5: helper is declared")
		require.Contains(t, revise[2].Content, "respond with only the complete file, as plain text without code fences or any other text.")
		require.NotContains(t, revise[2].Content, "previous response")

		require.Contains(t, s.Requests()[2][0].Content, "reworked")
	})

	t.Run("renders the source again from each revision", func(t *testing.T) {
		v := violationrecord(1, 5, "helper is declared")
		s := egllm.Constant(v, "reworked", v, "reworked again", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		var previous []string
		reprompt := func(p string) (string, error) {
			previous = append(previous, p)
			return "source for " + p, nil
		}

		_, ok, err := o.rework(t.Context(), "source", reprompt, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, []string{"reworked", "reworked again"}, previous)
		require.Equal(t, "source", s.Requests()[1][0].Content)
		require.Equal(t, "source for reworked", s.Requests()[3][0].Content)
	})

	t.Run("fails once the revisions are exhausted", func(t *testing.T) {
		v := violationrecord(1, 5, "helper is declared")
		s := egllm.Constant(v, "reworked", v, "reworked again", v)
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), "source", same, "generated")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.Requests(), 5)
	})

	t.Run("zero revisions fails without reworking", func(t *testing.T) {
		s := egllm.Constant(violationrecord(1, 5, "helper is declared"))
		o := option{ensure: ensure, llm: s}

		_, ok, err := o.rework(t.Context(), "source", same, "generated")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.Requests(), 1)
	})

	t.Run("an undecodable judgement uses a round", func(t *testing.T) {
		s := egllm.Constant("looks fine to me", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), "source", same, "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Len(t, s.Requests(), 2)
	})

	t.Run("numbers the rules and the response for the judge", func(t *testing.T) {
		s := egllm.Constant("")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, _, err := o.rework(t.Context(), "source", same, "first\nsecond")
		require.NoError(t, err)

		prompt := s.Requests()[0][0].Content
		require.True(t, strings.Contains(prompt, "1. no helper functions are added"))
		require.True(t, strings.Contains(prompt, "1: first\n2: second"))
	})
}
