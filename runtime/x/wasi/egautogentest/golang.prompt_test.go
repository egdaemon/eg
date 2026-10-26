package egautogentest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolangPrompt(t *testing.T) {
	fn := Fn{Path: ".fixtures/example.1/sample.go", Name: "Greet"}

	t.Run("includes the rules", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn, Ensure: []string{"no helper functions are added", "no global variables"}})
		require.NoError(t, err)
		require.Contains(t, prompt, "rules:\n- no helper functions are added\n- no global variables\n\n")
		require.NotContains(t, prompt, "{{")
	})

	t.Run("replaces the default rules when given rules", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn, Ensure: []string{"no helper functions are added"}})
		require.NoError(t, err)
		require.NotContains(t, prompt, "you must not include *any* comments")
	})

	t.Run("uses the default rules without rules", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		for _, rule := range golangDefaultRules {
			require.Contains(t, prompt, "- "+rule+"\n")
		}
	})

	t.Run("includes the style and the function", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Style("STYLE SAMPLE").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "STYLE SAMPLE")
		require.Contains(t, prompt, "func Greet")
	})

	t.Run("does not interpret template syntax within the source", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Style("{{.Fn.Name}} {{ bad").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "{{.Fn.Name}} {{ bad")
	})

	t.Run("includes the existing test file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.CopyFS(dir, os.DirFS(".fixtures/example.1")))
		fn := Fn{Path: filepath.Join(dir, "sample.go"), Name: "Greet"}
		name, err := golangFilename(DefaultGolangFilename, fn)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package sample\n\nfunc TestExisting() {}\n"), 0644))

		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "EXISTING TESTS")
		require.Contains(t, prompt, "func TestExisting() {}")
	})

	t.Run("omits the existing tests section without a test file", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.NotContains(t, prompt, "EXISTING TESTS")
	})

	t.Run("uses the configured file name to find the existing tests", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.CopyFS(dir, os.DirFS(".fixtures/example.1")))
		fn := Fn{Path: filepath.Join(dir, "sample.go"), Name: "Greet"}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "custom_test.go"), []byte("package sample\n\nfunc TestCustom() {}\n"), 0644))

		prompt, err := Golang(From(fn)).Filename("custom_test.go").Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)
		require.Contains(t, prompt, "func TestCustom() {}")
	})

	t.Run("unknown function", func(t *testing.T) {
		_, err := Golang(From(fn)).Prompt(PromptContext{Fn: Fn{Path: fn.Path, Name: "Missing"}})
		require.Error(t, err)
	})
}
