package egautogen

import (
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func resolveOptions(t Options) option {
	o := option{revisions: DefaultRevisions}
	for _, opt := range t {
		opt(&o)
	}

	return o
}

func TestNew(t *testing.T) {
	t.Run("defaults to no rules, one attempt and the default revisions", func(t *testing.T) {
		o := resolveOptions(New())
		require.Empty(t, o.ensure)
		require.Equal(t, 0, o.attempts)
		require.Equal(t, uint(DefaultRevisions), o.revisions)
	})

	t.Run("applies chained options", func(t *testing.T) {
		o := resolveOptions(New().Judge(egllm.Echo()).Attempts(3))
		require.Equal(t, egllm.Echo(), o.llm)
		require.Equal(t, 3, o.attempts)
	})

	t.Run("accumulates rules in order", func(t *testing.T) {
		require.Equal(t, []string{"a", "b", "c"}, resolveOptions(New().Ensure("a", "b").Ensure("c")).ensure)
	})

	t.Run("overrides revisions", func(t *testing.T) {
		require.Equal(t, uint(7), resolveOptions(New().Revisions(7)).revisions)
	})

	t.Run("allows zero revisions", func(t *testing.T) {
		require.Equal(t, uint(0), resolveOptions(New().Revisions(0)).revisions)
	})

	t.Run("chaining does not modify the receiver", func(t *testing.T) {
		a, b := egllm.Constant(), egllm.Constant()
		base := New().Judge(a)
		base.Judge(b)
		require.Same(t, a, resolveOptions(base).llm)
	})
}
