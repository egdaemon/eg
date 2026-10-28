package egautogentest

import (
	"strings"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestRework(t *testing.T) {
	ensure := []string{"no helper functions are added"}

	t.Run("keeps a response satisfying the rules without reworking it", func(t *testing.T) {
		s := script(t, "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		got, ok, err := o.rework(t.Context(), "source", "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "generated", got)
		require.Len(t, s.requests, 1)
	})

	t.Run("sends the source and the generated response back with the violations", func(t *testing.T) {
		s := script(t, violationrecord(1, 5, "helper is declared"), "reworked", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		got, ok, err := o.rework(t.Context(), "source", "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "reworked", got)
		require.Len(t, s.requests, 3)

		revise := s.requests[1]
		require.Len(t, revise, 3)
		require.Equal(t, egllm.Message{Role: "user", Content: "source"}, revise[0])
		require.Equal(t, egllm.Message{Role: "assistant", Content: "generated"}, revise[1])
		require.Equal(t, "user", revise[2].Role)
		require.Contains(t, revise[2].Content, "rule 1 (no helper functions are added) line 5: helper is declared")
		require.Contains(t, revise[2].Content, "respond with only the complete file, as plain text without code fences or any other text.")
		require.NotContains(t, revise[2].Content, "previous response")

		require.Contains(t, s.requests[2][0].Content, "reworked")
	})

	t.Run("fails once the revisions are exhausted", func(t *testing.T) {
		v := violationrecord(1, 5, "helper is declared")
		s := script(t, v, "reworked", v, "reworked again", v)
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), "source", "generated")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.requests, 5)
	})

	t.Run("zero revisions fails without reworking", func(t *testing.T) {
		s := script(t, violationrecord(1, 5, "helper is declared"))
		o := option{ensure: ensure, llm: s}

		_, ok, err := o.rework(t.Context(), "source", "generated")
		require.NoError(t, err)
		require.False(t, ok)
		require.Len(t, s.requests, 1)
	})

	t.Run("an undecodable judgement uses a round", func(t *testing.T) {
		s := script(t, "looks fine to me", "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, ok, err := o.rework(t.Context(), "source", "generated")
		require.NoError(t, err)
		require.True(t, ok)
		require.Len(t, s.requests, 2)
	})

	t.Run("numbers the rules and the response for the judge", func(t *testing.T) {
		s := script(t, "")
		o := option{ensure: ensure, revisions: 2, llm: s}

		_, _, err := o.rework(t.Context(), "source", "first\nsecond")
		require.NoError(t, err)

		prompt := s.requests[0][0].Content
		require.True(t, strings.Contains(prompt, "1. no helper functions are added"))
		require.True(t, strings.Contains(prompt, "1: first\n2: second"))
	})
}
