package compute

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/stretchr/testify/require"
)

func cacheKong(t *testing.T, cli any) *kong.Kong {
	t.Helper()
	k, err := kong.New(
		cli,
		cmdopts.RuntimeResources{Arch: "amd64", OS: "linux"}.KongVars(),
		kong.Vars{
			"vars_cache_directory":         t.TempDir(),
			"vars_eg_root_directory":       t.TempDir(),
			"vars_endpoint":                "https://localhost",
			"vars_git_default_remote_name": "origin",
			"vars_git_head_reference":      "HEAD",
			"vars_ssh_key_path":            filepath.Join(t.TempDir(), "id"),
		},
		kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
	)
	require.NoError(t, err)
	return k
}

func TestCacheArgs(t *testing.T) {
	t.Run("insert defaults maximum to 1GiB", func(t *testing.T) {
		var cli struct {
			Cache cmdCache `cmd:""`
		}
		path := filepath.Join(t.TempDir(), "a.txt")
		require.NoError(t, os.WriteFile(path, []byte("a"), 0600))

		_, err := cacheKong(t, &cli).Parse([]string{"cache", "insert", path})
		require.NoError(t, err)
		require.Equal(t, bytesx.Unit(bytesx.GiB), cli.Cache.Insert.Maximum)
		require.Equal(t, []string{path}, cli.Cache.Insert.Paths)
	})

	t.Run("insert accepts maximum and multiple paths", func(t *testing.T) {
		var cli struct {
			Cache cmdCache `cmd:""`
		}
		file := filepath.Join(t.TempDir(), "a.txt")
		require.NoError(t, os.WriteFile(file, []byte("a"), 0600))
		dir := t.TempDir()

		_, err := cacheKong(t, &cli).Parse([]string{"cache", "insert", "--maximum", "5GiB", file, dir})
		require.NoError(t, err)
		require.Equal(t, bytesx.Unit(5*bytesx.GiB), cli.Cache.Insert.Maximum)
		require.Equal(t, []string{file, dir}, cli.Cache.Insert.Paths)
	})

	t.Run("insert requires a path", func(t *testing.T) {
		var cli struct {
			Cache cmdCache `cmd:""`
		}
		_, err := cacheKong(t, &cli).Parse([]string{"cache", "insert"})
		require.Error(t, err)
	})

	t.Run("delete accepts cache relative paths", func(t *testing.T) {
		var cli struct {
			Cache cmdCache `cmd:""`
		}
		_, err := cacheKong(t, &cli).Parse([]string{"cache", "delete", "foo", "bar/baz"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar/baz"}, cli.Cache.Delete.Paths)
	})
}

func TestCachePayload(t *testing.T) {
	t.Run("packs files by basename and directories by contents", func(t *testing.T) {
		src := t.TempDir()
		file := filepath.Join(src, "a.txt")
		require.NoError(t, os.WriteFile(file, []byte("0123456789"), 0600))
		dir := filepath.Join(src, "dir")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "b.txt"), []byte("01234"), 0600))

		path, size, err := cachePayload(t.TempDir(), file, dir)
		require.NoError(t, err)
		require.Equal(t, cachePayloadArchive, filepath.Base(path))
		require.Equal(t, bytesx.Unit(15), size)

		archive, err := os.Open(path)
		require.NoError(t, err)
		defer archive.Close()

		dst := t.TempDir()
		require.NoError(t, tarx.Unpack(dst, archive))

		a, err := os.ReadFile(filepath.Join(dst, "a.txt"))
		require.NoError(t, err)
		require.Equal(t, "0123456789", string(a))

		b, err := os.ReadFile(filepath.Join(dst, "nested", "b.txt"))
		require.NoError(t, err)
		require.Equal(t, "01234", string(b))
	})

	t.Run("empty directory has zero size", func(t *testing.T) {
		_, size, err := cachePayload(t.TempDir(), t.TempDir())
		require.NoError(t, err)
		require.Zero(t, size)
	})

	t.Run("missing path fails", func(t *testing.T) {
		_, _, err := cachePayload(t.TempDir(), filepath.Join(t.TempDir(), "missing"))
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestCacheConfirmSize(t *testing.T) {
	unexpected := func(t *testing.T) func(string) (bool, error) {
		return func(string) (bool, error) {
			t.Fatal("unexpected confirmation prompt")
			return false, nil
		}
	}

	t.Run("below maximum does not prompt", func(t *testing.T) {
		require.NoError(t, cacheConfirmSize(bytesx.MiB, bytesx.GiB, unexpected(t)))
	})

	t.Run("at maximum does not prompt", func(t *testing.T) {
		require.NoError(t, cacheConfirmSize(bytesx.GiB, bytesx.GiB, unexpected(t)))
	})

	t.Run("above maximum confirmed", func(t *testing.T) {
		prompted := ""
		require.NoError(t, cacheConfirmSize(bytesx.GiB+1, bytesx.GiB, func(title string) (bool, error) {
			prompted = title
			return true, nil
		}))
		require.Contains(t, prompted, "exceeds --maximum")
	})

	t.Run("above maximum declined", func(t *testing.T) {
		err := cacheConfirmSize(bytesx.GiB+1, bytesx.GiB, func(string) (bool, error) {
			return false, nil
		})
		require.ErrorContains(t, err, "cancelled")
	})

	t.Run("prompt failure propagates", func(t *testing.T) {
		boom := errors.New("no tty")
		err := cacheConfirmSize(bytesx.GiB+1, bytesx.GiB, func(string) (bool, error) {
			return false, boom
		})
		require.ErrorIs(t, err, boom)
	})
}

func TestCacheDeletePaths(t *testing.T) {
	t.Run("cleans relative paths", func(t *testing.T) {
		cleaned, err := cacheDeletePaths("foo", "./bar", "baz/../qux/", "a//b")
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar", "qux", "a/b"}, cleaned)
	})

	for _, p := range []string{"..", "../foo", "foo/../../bar", "/etc", ".", "", "foo" + string(filepath.ListSeparator) + "bar"} {
		t.Run("rejects "+p, func(t *testing.T) {
			_, err := cacheDeletePaths("ok", p)
			require.Error(t, err)
		})
	}
}

func TestCacheBuiltinsEmbedded(t *testing.T) {
	for _, module := range []string{"cache/insert", "cache/delete"} {
		t.Run(module, func(t *testing.T) {
			_, err := fs.Stat(embeddedbuiltin, filepath.Join(".builtin", module, "main.go"))
			require.NoError(t, err)
		})
	}
}

// cacheDeleteScript extracts the removal script from the embedded cache/delete workload.
func cacheDeleteScript(t *testing.T) string {
	t.Helper()
	src, err := fs.ReadFile(embeddedbuiltin, ".builtin/cache/delete/main.go")
	require.NoError(t, err)

	f, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	require.NoError(t, err)

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if name.Name != "script" {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				require.True(t, ok)
				script, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				return script
			}
		}
	}

	require.FailNow(t, "script constant not found in cache/delete workload")
	return ""
}

func TestCacheDeleteScript(t *testing.T) {
	script := cacheDeleteScript(t)

	// layout: <tmp>/cache is the cache root, <tmp>/outside must never be touched.
	setup := func(t *testing.T) (cache, outside string) {
		root := t.TempDir()
		cache = filepath.Join(root, "cache")
		outside = filepath.Join(root, "outside")
		require.NoError(t, os.MkdirAll(filepath.Join(cache, "dir", "nested"), 0700))
		require.NoError(t, os.MkdirAll(outside, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(cache, "file.txt"), []byte("x"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(cache, "dir", "nested", "a.txt"), []byte("x"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(outside, "victim.txt"), []byte("x"), 0600))
		require.NoError(t, os.Symlink(outside, filepath.Join(cache, "escape")))
		require.NoError(t, os.Symlink(filepath.Join(outside, "victim.txt"), filepath.Join(cache, "victim.link")))
		require.NoError(t, os.Symlink("..", filepath.Join(cache, "up")))
		return cache, outside
	}

	run := func(t *testing.T, cache, path string) error {
		cmd := exec.Command("bash", "-o", "pipefail", "-c", script)
		cmd.Dir = cache
		cmd.Env = append(os.Environ(), "EG_CACHE_DELETE_PATH="+path)
		out, err := cmd.CombinedOutput()
		t.Log(string(out))
		return err
	}

	untouched := func(t *testing.T, outside string) {
		require.FileExists(t, filepath.Join(outside, "victim.txt"))
	}

	t.Run("removes a file", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "file.txt"))
		require.NoFileExists(t, filepath.Join(cache, "file.txt"))
		untouched(t, outside)
	})

	t.Run("removes a directory", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "dir"))
		require.NoDirExists(t, filepath.Join(cache, "dir"))
		untouched(t, outside)
	})

	t.Run("removes a nested path", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "dir/nested/a.txt"))
		require.NoFileExists(t, filepath.Join(cache, "dir", "nested", "a.txt"))
		require.DirExists(t, filepath.Join(cache, "dir", "nested"))
		untouched(t, outside)
	})

	t.Run("missing path is a noop", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "missing/entry"))
		untouched(t, outside)
	})

	t.Run("symlink to a directory is removed not followed", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "escape"))
		_, err := os.Lstat(filepath.Join(cache, "escape"))
		require.ErrorIs(t, err, fs.ErrNotExist)
		untouched(t, outside)
	})

	t.Run("symlink with trailing slash is removed not followed", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "escape/"))
		_, err := os.Lstat(filepath.Join(cache, "escape"))
		require.ErrorIs(t, err, fs.ErrNotExist)
		untouched(t, outside)
	})

	t.Run("symlink to a file is removed not followed", func(t *testing.T) {
		cache, outside := setup(t)
		require.NoError(t, run(t, cache, "victim.link"))
		_, err := os.Lstat(filepath.Join(cache, "victim.link"))
		require.ErrorIs(t, err, fs.ErrNotExist)
		untouched(t, outside)
	})

	for _, path := range []string{
		"escape/victim.txt", // through a symlink pointing outside.
		"up/outside",        // through a symlink to the parent.
		"up/cache",          // the cache root itself via its parent.
		"../outside",        // plain traversal.
		"dir/../../outside", // traversal after a real directory.
		"/tmp",              // absolute.
		".",                 // the cache root.
		"..",                // the parent of the cache root.
		"dir/..",            // the cache root via traversal.
	} {
		t.Run("refuses "+path, func(t *testing.T) {
			cache, outside := setup(t)
			require.Error(t, run(t, cache, path))
			untouched(t, outside)
			require.DirExists(t, filepath.Join(cache, "dir"))
			require.FileExists(t, filepath.Join(cache, "file.txt"))
		})
	}
}
