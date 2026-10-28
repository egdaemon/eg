package egautogentest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolangPackages(t *testing.T) {
	fn := Fn{Path: ".fixtures/golang/example.3/sample.go", Name: "Noop"}

	t.Run("lists the exported functions of a package within the module", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Templates("import \"example.com/mod/api\"\n\nfunc TestX() {}").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Less(t, strings.Index(prompt, "TYPES"), strings.Index(prompt, "package api // import"))
		require.Less(t, strings.Index(prompt, "package api // import"), strings.Index(prompt, "PACKAGE"))
		require.Contains(t, prompt, "package api // import \"example.com/mod/api\"\n")
		require.Contains(t, prompt, "func Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool\n")
		require.Contains(t, prompt, "func Zero[T any](v T) bool")
	})

	t.Run("skips documentation, bodies, printf variants, unexported functions, methods and tests", func(t *testing.T) {
		signatures, err := packageSignatures(".fixtures/golang/example.3/api")
		require.NoError(t, err)
		require.Equal(t, "func Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool\nfunc Zero[T any](v T) bool", signatures)
	})

	t.Run("finds packages in the module cache", func(t *testing.T) {
		cache := t.TempDir()
		t.Setenv("GOMODCACHE", cache)

		dir := filepath.Join(cache, "example.org", "!cached@v1.0.0", "sub")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub.go"), []byte("package sub\n\nfunc Cached() {}\n"), 0o600))

		prompt, err := Golang(From(fn)).Templates("import \"example.org/Cached/sub\"\n\nfunc TestX() {}").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "func Cached()")
	})

	t.Run("omits the section without packages", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.NotContains(t, prompt, "// import")
	})

	t.Run("discovers packages imported by the templates", func(t *testing.T) {
		template := "import (\n\t\"testing\"\n\n\t\"example.com/mod/api\"\n\t\"example.net/missing\"\n)\n\nfunc TestX(t *testing.T) {}"
		prompt, err := Golang(From(fn)).Templates(template).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "package api // import \"example.com/mod/api\"\n")
		require.NotContains(t, prompt, "// import \"example.net/missing\"")
	})

	t.Run("discovers packages from a full file template", func(t *testing.T) {
		template := "package x\n\nimport \"example.org/lib\"\n\nvar _ = lib.Vendored"
		prompt, err := Golang(From(fn)).Templates(template).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "func Vendored(name string) string")
	})
}
