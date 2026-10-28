package egautogentest

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestGolang(t *testing.T) {
	t.Run("binds the sequence", func(t *testing.T) {
		seq := From(Fn{Name: "Greet"})
		require.Equal(t, seq, Golang(seq).Seq())
	})

	t.Run("applies templates", func(t *testing.T) {
		require.Equal(t, []string{"s"}, Golang(From()).Templates("s").resolve().templates)
	})

	t.Run("templates accumulate", func(t *testing.T) {
		require.Equal(t, []string{"a", "b", "c"}, Golang(From()).Templates("a", "b").Templates("c").resolve().templates)
	})

	t.Run("chaining does not modify the receiver", func(t *testing.T) {
		base := Golang(From()).Templates("a")
		base.Templates("b")
		require.Equal(t, []string{"a"}, base.resolve().templates)
	})
}

func TestGolangDirectory(t *testing.T) {
	t.Run("defaults to the path as is", func(t *testing.T) {
		require.Equal(t, "a/b.go", Golang(From()).resolve().rebase("a/b.go"))
		require.Equal(t, "/ws/a/b.go", Golang(From()).resolve().rebase("/ws/a/b.go"))
	})

	t.Run("relative paths are resolved against the directory", func(t *testing.T) {
		require.Equal(t, "/wt/a/b.go", Golang(From()).Directory("/wt").resolve().rebase("a/b.go"))
	})

	t.Run("absolute workspace paths are moved into the directory", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")
		o := Golang(From()).Directory("/wt").resolve()
		require.Equal(t, "/wt/a/b.go", o.rebase("/ws/a/b.go"))
		require.Equal(t, "/elsewhere/b.go", o.rebase("/elsewhere/b.go"))
	})
}
