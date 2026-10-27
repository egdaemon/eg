package egautogentest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolangPackages(t *testing.T) {
	fn := Fn{Path: ".fixtures/example.3/sample.go", Name: "Noop"}

	t.Run("lists the exported functions of a package within the module", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Packages("example.com/mod/api").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Less(t, strings.Index(prompt, "TYPES"), strings.Index(prompt, "package api // import"))
		require.Less(t, strings.Index(prompt, "package api // import"), strings.Index(prompt, "PACKAGE"))
		require.Contains(t, prompt, "package api // import \"example.com/mod/api\"\n")
		require.Contains(t, prompt, "func Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool\n")
		require.Contains(t, prompt, "func Zero[T any](v T) bool")
	})

	t.Run("skips documentation, bodies, printf variants, unexported functions, methods and tests", func(t *testing.T) {
		signatures, err := packageSignatures(".fixtures/example.3/api")
		require.NoError(t, err)
		require.Equal(t, "func Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool\nfunc Zero[T any](v T) bool", signatures)
	})

	t.Run("finds vendored packages", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Packages("example.org/lib").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "func Vendored(name string) string")
	})

	t.Run("finds packages in the module cache", func(t *testing.T) {
		cache := t.TempDir()
		t.Setenv("GOMODCACHE", cache)

		dir := filepath.Join(cache, "example.org", "!cached@v1.0.0", "sub")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub.go"), []byte("package sub\n\nfunc Cached() {}\n"), 0o600))

		prompt, err := Golang(From(fn)).Packages("example.org/Cached/sub").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "func Cached()")
	})

	t.Run("fails for packages that cannot be found", func(t *testing.T) {
		_, err := Golang(From(fn)).Packages("example.net/missing").Prompt(PromptContext{Fn: fn})
		require.Error(t, err)
	})

	t.Run("omits the section without packages", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.NotContains(t, prompt, "// import")
	})
}
