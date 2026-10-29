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
		require.Less(t, strings.Index(prompt, "     TYPES      "), strings.Index(prompt, "package api // import"))
		require.Less(t, strings.Index(prompt, "package api // import"), strings.Index(prompt, "     SOURCE     "))
		require.Contains(t, prompt, "package api // import \"example.com/mod/api\"\n")
		require.Contains(t, prompt, "func Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool\n")
		require.Contains(t, prompt, "func Zero[T any](v T) bool")
	})

	t.Run("skips documentation, bodies, printf variants, unexported functions, methods and tests", func(t *testing.T) {
		signatures, err := packageSignatures(".fixtures/golang/example.3/api")
		require.NoError(t, err)
		require.Equal(t, "type Assertions struct{}\n\nfunc Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool\nfunc Zero[T any](v T) bool", signatures)
	})

	t.Run("declares the exported structs and interfaces in full, without what a caller can not use", func(t *testing.T) {
		dir := t.TempDir()
		src := "package kinds\n\n// Doc is not listed.\ntype Config struct {\n\tName string\n\tretries int\n}\n\ntype Reader interface {\n\tRead() string\n\tclose()\n}\n\ntype hidden struct{}\n\nfunc New() Config { return Config{} }\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "kinds.go"), []byte(src), 0o600))

		signatures, err := packageSignatures(dir)
		require.NoError(t, err)
		require.Equal(t, "type Config struct {\n\tName string\n\t// contains filtered or unexported fields\n}\n\ntype Reader interface {\n\tRead() string\n\t// contains filtered or unexported methods\n}\n\nfunc New() Config", signatures)
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

	t.Run("discovers packages imported by earlier attempts", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn, Previous: "package x_test\n\nimport \"example.com/mod/api\"\n\nvar _ = api.Zero[int]"})
		require.NoError(t, err)
		require.Contains(t, prompt, "package api // import \"example.com/mod/api\"\n")
	})

	t.Run("discovers standard library packages", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn, Previous: "package x_test\n\nimport \"crypto/ed25519\"\n\nvar _ = ed25519.GenerateKey"})
		require.NoError(t, err)
		require.Contains(t, prompt, "package ed25519 // import \"crypto/ed25519\"\n")
		require.Contains(t, prompt, "func GenerateKey(random io.Reader) (PublicKey, PrivateKey, error)")
	})

	t.Run("orders the packages by import path, the function's own package among them", func(t *testing.T) {
		template := "import (\n\t\"example.com/mod/api\"\n\t\"crypto/ed25519\"\n)\n\nvar _ = ed25519.GenerateKey"
		prompt, err := Golang(From(fn)).Templates(template).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)

		own, std, api := strings.Index(prompt, "package mod // import"), strings.Index(prompt, "package ed25519 // import"), strings.Index(prompt, "package api // import")
		require.NotEqual(t, -1, std)
		require.NotEqual(t, -1, api)
		require.Less(t, std, api)
		if own != -1 {
			require.Less(t, own, api)
		}
	})
}
