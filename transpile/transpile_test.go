package transpile_test

import (
	"crypto/md5"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

// regenerate the expected output of the fixtures: go test ./transpile -update
var update = flag.Bool("update", false, "update the expected transpiled output")

// each fixture is a directory of the form:
//
//	<name>/.eg/main.go      the module to transpile.
//	<name>/transpiled/...   the expected generated go files, relative to the transpiled tree.
//	<name>/error            optional, a message the transpile is expected to fail with instead.
func TestTranspile(t *testing.T) {
	fixtures, err := fs.Glob(os.DirFS(testx.Fixture()), "example.*")
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			srcdir := t.TempDir()
			tmpdir := t.TempDir()
			ws, err := workspaces.New(ctx, md5.New(), tmpdir, tmpdir, "")
			require.NoError(t, err)

			require.NoError(t, fsx.CloneTree(ctx, srcdir, filepath.Join(name, eg.DefaultModuleDirectory()), os.DirFS(testx.Fixture())))

			_, err = transpile.Autodetect(transpile.New(srcdir, ws)).Run(ctx)

			// fixtures with an error file expect the transpile to fail with the given message.
			if expectederr, rerr := os.ReadFile(testx.Fixture(name, "error")); rerr == nil {
				require.ErrorContains(t, err, strings.TrimSpace(string(expectederr)))
				return
			}
			require.NoError(t, err)

			actual := goFiles(t, filepath.Join(tmpdir, ws.TransDir))
			expecteddir := testx.Fixture(name, "transpiled")

			if *update {
				require.NoError(t, os.RemoveAll(expecteddir))
				for rel, content := range actual {
					dst := filepath.Join(expecteddir, rel)
					require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0750))
					require.NoError(t, os.WriteFile(dst, []byte(content), 0640))
				}
			}

			expected := goFiles(t, expecteddir)
			require.Equal(t, keys(expected), keys(actual), "generated files differ")
			for rel, content := range expected {
				require.Equal(t, content, actual[rel], rel)
			}
		})
	}
}

// goFiles reads all the go files in the tree keyed by their relative path.
func goFiles(t *testing.T, root string) map[string]string {
	found := make(map[string]string)
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found[rel] = testx.ReadString(path)
		return nil
	}))
	return found
}

func keys(m map[string]string) (r []string) {
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
