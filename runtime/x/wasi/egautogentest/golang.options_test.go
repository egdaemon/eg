package egautogentest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func sources(t *testing.T, o GolangOptions) []string {
	t.Helper()
	v, err := o.resolve().sources()
	require.NoError(t, err)
	return v
}

func TestGolang(t *testing.T) {
	t.Run("binds the sequence", func(t *testing.T) {
		seq := FromSpecified(Fn{Name: "Greet"})
		require.Equal(t, seq, Golang(seq).Seq())
	})

	t.Run("applies templates", func(t *testing.T) {
		require.Equal(t, []string{"s"}, sources(t, Golang(FromSpecified()).Templates("s")))
	})

	t.Run("templates accumulate", func(t *testing.T) {
		require.Equal(t, []string{"a", "b", "c"}, sources(t, Golang(FromSpecified()).Templates("a", "b").Templates("c")))
	})

	t.Run("applies template functions", func(t *testing.T) {
		fn := func() (string, error) { return "f", nil }
		require.Equal(t, []string{"s", "f"}, sources(t, Golang(FromSpecified()).Templates("s").Templates(fn)))
	})

	t.Run("applies named string types", func(t *testing.T) {
		type source string
		require.Equal(t, []string{"n"}, sources(t, Golang(FromSpecified()).Templates(source("n"))))
	})

	t.Run("skips empty templates", func(t *testing.T) {
		require.Equal(t, []string{"a"}, sources(t, Golang(FromSpecified()).Templates("", "a")))
	})

	t.Run("template function errors surface", func(t *testing.T) {
		_, err := Golang(FromSpecified()).Templates(func() (string, error) { return "", errors.New("boom") }).resolve().sources()
		require.ErrorContains(t, err, "boom")
	})

	t.Run("chaining does not modify the receiver", func(t *testing.T) {
		base := Golang(FromSpecified()).Templates("a")
		base.Templates("b")
		require.Equal(t, []string{"a"}, sources(t, base))
	})
}

func TestGolangDirectory(t *testing.T) {
	t.Run("defaults to the path as is", func(t *testing.T) {
		require.Equal(t, "a/b.go", Golang(FromSpecified()).resolve().rebase("a/b.go"))
		require.Equal(t, "/ws/a/b.go", Golang(FromSpecified()).resolve().rebase("/ws/a/b.go"))
	})

	t.Run("relative paths are resolved against the directory", func(t *testing.T) {
		require.Equal(t, "/wt/a/b.go", Golang(FromSpecified()).Directory("/wt").resolve().rebase("a/b.go"))
	})

	t.Run("absolute workspace paths are moved into the directory", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")
		o := Golang(FromSpecified()).Directory("/wt").resolve()
		require.Equal(t, "/wt/a/b.go", o.rebase("/ws/a/b.go"))
		require.Equal(t, "/elsewhere/b.go", o.rebase("/elsewhere/b.go"))
	})

	t.Run("the workspace is configurable", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")
		o := Golang(FromSpecified()).Directory("/wt").Workspace("/workload/eg").resolve()
		require.Equal(t, "/wt/authn/authn.go", o.rebase("/workload/eg/authn/authn.go"))
		require.Equal(t, "/ws/a/b.go", o.rebase("/ws/a/b.go"))
	})
}

func TestGolangLazy(t *testing.T) {
	t.Run("has no sequence until bound", func(t *testing.T) {
		require.Nil(t, GolangLazy().Seq())

		seq := FromSpecified(Fn{Name: "Greet"})
		require.Equal(t, seq, GolangLazy().From(seq).Seq())
	})

	t.Run("binding does not modify the receiver or other bindings", func(t *testing.T) {
		base := GolangLazy().Templates("a")
		a, b := FromSpecified(Fn{Name: "A"}), FromSpecified(Fn{Name: "B"})

		bound := base.From(a)
		other := base.From(b)

		require.Nil(t, base.Seq())
		require.Equal(t, a, bound.Seq())
		require.Equal(t, b, other.Seq())
		require.Equal(t, []string{"a"}, sources(t, bound))
	})

	t.Run("bound copies share the caches", func(t *testing.T) {
		base := GolangLazy()
		a, b := base.From(FromSpecified()).resolve(), base.From(FromSpecified()).resolve()

		require.Same(t, a.pkgs, b.pkgs)
		require.Same(t, a.callers, b.callers)
	})

	t.Run("loads a template function once", func(t *testing.T) {
		var calls atomic.Int32
		base := GolangLazy().Templates(func() (string, error) {
			calls.Add(1)
			return "loaded", nil
		})

		for range 5 {
			require.Equal(t, []string{"loaded"}, sources(t, base.From(FromSpecified())))
		}

		require.EqualValues(t, 1, calls.Load())
	})

	t.Run("bound concurrently", func(t *testing.T) {
		// a base with spare capacity, binding by append would write into the same array.
		base := GolangLazy().Templates("a", "b", "c").Templates("d")
		require.Greater(t, cap(base), len(base))

		var wg sync.WaitGroup
		for i := range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				seq := FromSpecified(Fn{Name: fmt.Sprintf("fn%d", i)})
				bound := base.From(seq)
				require.Equal(t, seq, bound.Seq())
				require.Equal(t, []string{"a", "b", "c", "d"}, sources(t, bound))
			}()
		}
		wg.Wait()
	})

	t.Run("caches load concurrently", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "p.go"), []byte("package p\n"), 0600))

		cache := GolangLazy().resolve()

		var wg sync.WaitGroup
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				pkg, err := cache.pkgs.load(dir)
				require.NoError(t, err)
				require.Equal(t, "p", pkg.name)
			}()
		}
		wg.Wait()
	})
}

func TestGolangLazyOptionsConcurrently(t *testing.T) {
	t.Run("options applied to a shared base do not interfere", func(t *testing.T) {
		base := GolangLazy().Templates("a", "b", "c").Templates("d")
		require.Greater(t, cap(base), len(base))

		var wg sync.WaitGroup
		for i := range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				dir := fmt.Sprintf("/tmp/%d", i)
				o := base.Directory(dir).Filename(fmt.Sprintf("%d_test.go", i)).From(FromSpecified()).resolve()
				require.Equal(t, dir, o.dir)
				require.Equal(t, fmt.Sprintf("%d_test.go", i), o.filename)
			}()
		}
		wg.Wait()
	})
}
