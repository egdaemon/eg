package egautogentest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolangCallers(t *testing.T) {
	newcache := func() *callcache { return &callcache{built: map[string]*callindex{}} }

	t.Run("finds callers in the same package first and then in the rest of the module", func(t *testing.T) {
		calls, err := newcache().callers(".fixtures/example.4/lib", "Target")
		require.NoError(t, err)
		require.Len(t, calls, 3)
		require.Contains(t, calls[0], "value := Target()")
		require.Contains(t, calls[0], "// lib/lib.go\n")
		require.Contains(t, calls[1], "var Name = mylib.Target()")
		require.Contains(t, calls[2], "v := lib.Target()")
	})

	t.Run("does not match functions of other packages with the same name", func(t *testing.T) {
		calls, err := newcache().callers(".fixtures/example.4/lib", "Target")
		require.NoError(t, err)
		for _, call := range calls {
			require.NotContains(t, call, "other/other.go")
			require.NotContains(t, call, "vendor")
		}
	})

	t.Run("finds only the callers of the package the function belongs to", func(t *testing.T) {
		calls, err := newcache().callers(".fixtures/example.4/other", "Target")
		require.NoError(t, err)
		require.Len(t, calls, 1)
		require.Contains(t, calls[0], "return Target()")
	})

	t.Run("reports nothing for functions without callers and for methods", func(t *testing.T) {
		calls, err := newcache().callers(".fixtures/example.4/lib", "Local")
		require.NoError(t, err)
		require.Empty(t, calls)

		calls, err = newcache().callers(".fixtures/example.4/lib", "Type.Method")
		require.NoError(t, err)
		require.Empty(t, calls)
	})

	t.Run("builds the index once per module", func(t *testing.T) {
		cache := newcache()
		_, err := cache.callers(".fixtures/example.4/lib", "Target")
		require.NoError(t, err)
		require.Len(t, cache.built, 1)
		_, err = cache.callers(".fixtures/example.4/other", "Target")
		require.NoError(t, err)
		require.Len(t, cache.built, 1)
	})

	t.Run("the prompt includes the call sites", func(t *testing.T) {
		fn := Fn{Path: ".fixtures/example.4/lib/lib.go", Name: "Target"}
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "call sites within the module:")
		require.Contains(t, prompt, "lib.Target()")
		require.NotContains(t, prompt, "no existing usage")
	})

	t.Run("long call sites are cut short", func(t *testing.T) {
		long := strings.Repeat("x\n", maxCallerLines+5)
		cut := truncateLines(long, maxCallerLines)
		require.Equal(t, maxCallerLines+1, strings.Count(cut, "\n")+1)
		require.True(t, strings.HasSuffix(cut, "// ..."))
	})
}
