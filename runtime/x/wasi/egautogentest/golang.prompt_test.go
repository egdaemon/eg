package egautogentest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestGolangPrompt(t *testing.T) {
	fn := Fn{Path: ".fixtures/golang/example.1/sample.go", Name: "Greet"}

	// copy places a copy of example.1 in a temporary directory, so the tests next to it can be changed.
	copy := func(t *testing.T) (string, Fn) {
		dir := t.TempDir()
		require.NoError(t, os.CopyFS(dir, os.DirFS(".fixtures/golang/example.1")))
		return dir, Fn{Path: filepath.Join(dir, "sample.go"), Name: "Greet"}
	}

	write := func(t *testing.T, dir string, name string, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}

	for _, tc := range []struct {
		name    string
		fixture string
		setup   func(t *testing.T) (PromptContext, GolangOptions)
	}{
		{"example.1", "example.1", func(t *testing.T) (PromptContext, GolangOptions) {
			return PromptContext{Fn: fn}, Golang(From(fn))
		}},
		{"example.2", "example.2", func(t *testing.T) (PromptContext, GolangOptions) {
			shout := Fn{Path: ".fixtures/golang/example.2/sample.go", Name: "Shout"}
			return PromptContext{Fn: shout}, Golang(From(shout))
		}},
		{"example.2.private", "example.2.private", func(t *testing.T) (PromptContext, GolangOptions) {
			quiet := Fn{Path: ".fixtures/golang/example.2/sample.go", Name: "quiet"}
			return PromptContext{Fn: quiet}, Golang(From(quiet))
		}},
		{"example.5", "example.5", func(t *testing.T) (PromptContext, GolangOptions) {
			exported := Fn{Path: ".fixtures/golang/example.5/sample.go", Name: "Exported"}
			return PromptContext{Fn: exported}, Golang(From(exported))
		}},
		{"example.5.method", "example.5.method", func(t *testing.T) (PromptContext, GolangOptions) {
			method := Fn{Path: ".fixtures/golang/example.5/sample.go", Name: "Generic.Method"}
			return PromptContext{Fn: method}, Golang(From(method))
		}},
		{"example.6", "example.6", func(t *testing.T) (PromptContext, GolangOptions) {
			run := Fn{Path: ".fixtures/golang/example.6/main.go", Name: "Run"}
			return PromptContext{Fn: run}, Golang(From(run))
		}},
		{"example.1.ensure", "example.1.ensure", func(t *testing.T) (PromptContext, GolangOptions) {
			return PromptContext{Fn: fn, Ensure: []string{"no helper functions are added", golangDefaultRules[1], "no global variables"}}, Golang(From(fn))
		}},
		{"example.1.templates", "example.1.templates", func(t *testing.T) (PromptContext, GolangOptions) {
			return PromptContext{Fn: fn}, Golang(From(fn)).Templates("STYLE SAMPLE", "{{.Fn.Name}} {{ bad")
		}},
		{"example.1.generated", "example.1.generated", func(t *testing.T) (PromptContext, GolangOptions) {
			dir, fn := copy(t)
			require.NoError(t, os.Remove(filepath.Join(dir, "sample_test.go")))
			name, err := golangFilename(DefaultGolangFilename, fn)
			require.NoError(t, err)
			write(t, dir, name, "package sample\n\nfunc TestExisting() {}\n")
			return PromptContext{Fn: fn}, Golang(From(fn))
		}},
		{"prefers the test file next to the function", "example.1", func(t *testing.T) (PromptContext, GolangOptions) {
			dir, fn := copy(t)
			name, err := golangFilename(DefaultGolangFilename, fn)
			require.NoError(t, err)
			write(t, dir, name, "package sample\n\nfunc TestGenerated() {}\n")
			return PromptContext{Fn: fn}, Golang(From(fn))
		}},
		{"example.1.external", "example.1.external", func(t *testing.T) (PromptContext, GolangOptions) {
			dir, fn := copy(t)
			write(t, dir, "sample_test.go", "package sample_test\n\nfunc TestExternal() {}\n")
			return PromptContext{Fn: fn}, Golang(From(fn))
		}},
		{"example.1.custom", "example.1.custom", func(t *testing.T) (PromptContext, GolangOptions) {
			dir, fn := copy(t)
			require.NoError(t, os.Remove(filepath.Join(dir, "sample_test.go")))
			write(t, dir, "custom_test.go", "package sample\n\nfunc TestCustom() {}\n")
			return PromptContext{Fn: fn}, Golang(From(fn)).Filename("custom_test.go")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, g := tc.setup(t)
			prompt, err := g.Prompt(c)
			require.NoError(t, err)

			expected, err := os.ReadFile(filepath.Join(".fixtures", "golang", tc.fixture+".prompt.md"))
			require.NoError(t, err)
			require.Equal(t, string(expected), prompt)
		})
	}

	t.Run("rules are the default rules followed by the given rules", func(t *testing.T) {
		rules := Golang(From(fn)).Rules("mine", golangDefaultRules[0])
		require.Equal(t, append(append([]string(nil), golangDefaultRules...), "mine"), rules)
	})

	t.Run("unknown function", func(t *testing.T) {
		_, err := Golang(From(fn)).Prompt(PromptContext{Fn: Fn{Path: fn.Path, Name: "Missing"}})
		require.Error(t, err)
	})
}

func TestGolangPromptFeedback(t *testing.T) {
	fn := Fn{Path: ".fixtures/golang/example.1/sample.go", Name: "Greet"}
	order := func(prompt string, labels ...string) (positions []int) {
		for _, label := range labels {
			if loc := regexp.MustCompile(`-+ +` + label + ` +-+`).FindStringIndex(prompt); loc != nil {
				positions = append(positions, loc[0])
			} else {
				positions = append(positions, -1)
			}
		}
		return positions
	}

	t.Run("sections are in order and the feedback is left out of a first attempt", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{Fn: fn})
		require.NoError(t, err)

		positions := order(prompt, "RULES", "TEMPLATES", "TYPES", "SOURCE", "PREAMBLE", "FOCUS")
		require.NotContains(t, positions, -1)
		require.IsIncreasing(t, positions)
		require.Equal(t, []int{-1, -1, -1}, order(prompt, "PREVIOUS", "VIOLATIONS", "VERIFY RESULTS"))
	})

	t.Run("the previous response, its violations and its verification results follow the focus", func(t *testing.T) {
		prompt, err := Golang(From(fn)).Prompt(PromptContext{
			Fn:         fn,
			Previous:   "package sample_test\nfunc TestGreet() {}",
			Violations: "- rule 1 (be clean) line 2: dirty\n",
			Results:    "FAIL: undefined: x",
		})
		require.NoError(t, err)

		positions := order(prompt, "FOCUS", "PREVIOUS", "VIOLATIONS", "VERIFY RESULTS")
		require.NotContains(t, positions, -1)
		require.IsIncreasing(t, positions)
		require.Contains(t, prompt, "1: package sample_test\n2: func TestGreet() {}\n")
		require.Contains(t, prompt, "- rule 1 (be clean) line 2: dirty\n")
		require.Contains(t, prompt, "FAIL: undefined: x\n")
	})
}

func TestFormatPreamble(t *testing.T) {
	require.Equal(t, "package main\n\nimport (\n\t// required imports\n)\n\n// DO NOT EDIT: This file was auto generated by egautotest.", formatPreamble("main"))
}

func TestWithResults(t *testing.T) {
	failure := errors.New("failing")
	write := func(t *testing.T, content string) string {
		path := filepath.Join(t.TempDir(), "output")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	t.Run("adds the output to the error", func(t *testing.T) {
		err := withResults(failure, write(t, "# pkg\nx.go:1: undefined\n"))
		require.ErrorIs(t, err, failure)
		require.Equal(t, "failing\n# pkg\nx.go:1: undefined", err.Error())
	})

	t.Run("keeps the error without output", func(t *testing.T) {
		require.Equal(t, failure, withResults(failure, write(t, " \n")))
		require.Equal(t, failure, withResults(failure, filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("cuts long output short", func(t *testing.T) {
		err := withResults(failure, write(t, strings.Repeat("x", maxResults+100)))
		require.Equal(t, len("failing\n")+maxResults+len("\n..."), len(err.Error()))
		require.True(t, strings.HasSuffix(err.Error(), "\n..."))
	})
}

func TestCleanImports(t *testing.T) {
	t.Run("runs goimports on the file", func(t *testing.T) {
		var (
			cmd = shell.NewLocal()
			rec = shell.NewRecorder(&cmd)
		)

		cleanImports(t.Context(), cmd, "/dir/.autogentest.1")
		require.Equal(t, []string{"::bash:-c goimports -w '/dir/.autogentest.1'"}, rec.Results())
	})

	t.Run("a failure is not an error", func(t *testing.T) {
		cmd := shell.NewLocal().UnsafeExec(func(context.Context, string, []string, string, []string) error {
			return errors.New("goimports: command not found")
		})

		require.NotPanics(t, func() { cleanImports(t.Context(), cmd, "/dir/.autogentest.1") })
	})
}
