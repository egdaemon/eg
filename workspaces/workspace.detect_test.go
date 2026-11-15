package workspaces_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestDetect(t *testing.T) {
	t.Run("parse egrc skips blank lines and comments", func(t *testing.T) {
		require.Equal(t, []string{"--gcp-auto", "--ttl=infinity"}, workspaces.ParseEgrc("# comment\n\n  --gcp-auto  \n--ttl=infinity\n   # indented comment\n"))
		require.Empty(t, workspaces.ParseEgrc("\n# nothing\n"))
	})

	t.Run("expand env", func(t *testing.T) {
		env := map[string]string{"USER": "me", "A_B": "x"}

		for input, expected := range map[string]string{
			"env-$USER-${A_B}": "env-me-x",
			"a-${UNSET}-b":     "a--b",
			"$$USER costs $5":  "$USER costs $5",
			"${USER}_":         "me_",
		} {
			require.Equal(t, expected, workspaces.ExpandEnv(input, func(name string) string { return env[name] }), input)
		}
	})

	t.Run("read detected modes", func(t *testing.T) {
		type way struct {
			mode workspaces.Mode
			args []string
		}

		for name, c := range map[string]struct {
			present  bool
			egrc     string
			expected []way
		}{
			"no .egrc is local only":           {expected: []way{{workspaces.ModeLocal, nil}}},
			"sectionless is local only":        {present: true, egrc: "# args\n--gcp-auto\n", expected: []way{{workspaces.ModeLocal, []string{"--gcp-auto"}}}},
			"serve only":                       {present: true, egrc: "[serve]\n--ttl=infinity\n--secret=x-${USER}\n", expected: []way{{workspaces.ModeServe, []string{"--ttl=infinity", "--secret=x-me"}}}},
			"modes without arguments":          {present: true, egrc: "[local]\n[upload]\n", expected: []way{{workspaces.ModeLocal, nil}, {workspaces.ModeUpload, nil}}},
			"common arguments lead every mode": {present: true, egrc: "--gcp-auto\n[local]\n--vram=32GiB\n[upload]\n--secret=s\n", expected: []way{{workspaces.ModeLocal, []string{"--gcp-auto", "--vram=32GiB"}}, {workspaces.ModeUpload, []string{"--gcp-auto", "--secret=s"}}}},
			"baremetal":                        {present: true, egrc: "[baremetal]\n--ttl=infinity\n", expected: []way{{workspaces.ModeBaremetal, []string{"--ttl=infinity"}}}},
			"a repeated header appends":        {present: true, egrc: "[local]\n--a\n[upload]\n[local]\n--b\n", expected: []way{{workspaces.ModeLocal, []string{"--a", "--b"}}, {workspaces.ModeUpload, nil}}},
		} {
			dir := t.TempDir()
			if c.present {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".egrc"), []byte(c.egrc), 0600), name)
			}

			found, err := workspaces.ReadDetected(dir, func(string) string { return "me" })
			require.NoError(t, err, name)

			actual := make([]way, 0, len(found))
			for _, d := range found {
				require.Equal(t, dir, d.Dir, name)
				actual = append(actual, way{d.Mode, d.Args})
			}
			require.ElementsMatch(t, c.expected, actual, name)
		}

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".egrc"), []byte("[remote]\n"), 0600))
		_, err := workspaces.ReadDetected(dir, func(string) string { return "" })
		require.ErrorContains(t, err, "unknown mode [remote]")
	})

	t.Run("ignored", func(t *testing.T) {
		root := t.TempDir()

		for rel, content := range map[string]string{
			".eg/.egrcignore":                   "# hide the dev workloads except the daemon\ndev/*\n!dev/daemon\n",
			".eg/experiments/.egrcignore":       "scratch-*/\n*.draft/\n",
			".eg/dev/console/main.go":           "package main\n",
			".eg/dev/gcpaccess/main.go":         "package main\n",
			".eg/dev/daemon/main.go":            "package main\n",
			".eg/experiments/scratch-1/main.go": "package main\n",
			".eg/experiments/keep/main.go":      "package main\n",
			".eg/experiments/a.draft/main.go":   "package main\n",
			".eg/other/.egrcignore":             "",
			".eg/other/main.go":                 "package main\n",
			".eg/other/nested/main.go":          "package main\n",
			".eg/sibling/main.go":               "package main\n",
		} {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0600))
		}

		for rel, expected := range map[string]bool{
			".eg/dev/console":           true,
			".eg/dev/gcpaccess":         true,
			".eg/dev/daemon":            false,
			".eg/experiments/scratch-1": true,
			".eg/experiments/a.draft":   true,
			".eg/experiments/keep":      false,
			// an empty file ignores its own directory and what is below it.
			".eg/other":        true,
			".eg/other/nested": true,
			// patterns of one subtree do not leak into the others.
			".eg/sibling": false,
			".eg":         false,
		} {
			result, err := workspaces.Ignored(root, filepath.Join(root, rel))
			require.NoError(t, err)
			require.Equal(t, expected, result, rel)
		}
	})

	t.Run("detect workloads", func(t *testing.T) {
		root := t.TempDir()

		for rel, content := range map[string]string{
			"go.mod":                  "module fixture\n\ngo 1.24\n",
			".eg/main.go":             "package main\n\nfunc main() {}\n",
			".eg/a/main.go":           "package main\n\nfunc main() {}\n",
			".eg/a/.egrc":             "[local]\n--gcp-auto\n[upload]\n",
			".eg/b/main.go":           "package main\n\nfunc main() {}\n",
			".eg/b/.egrc":             "[serve]\n--ttl=infinity\n--secret=x-${USER}\n",
			".eg/helper/helper.go":    "package helper\n",
			".eg/ignored/main.go":     "package main\n\nfunc main() {}\n",
			".eg/ignored/.egrcignore": "",
		} {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0600))
		}

		seq := workspaces.DetectWorkloads(root, func(string) string { return "me" })
		var found []workspaces.Detected
		for d := range seq.Each(t.Context()) {
			found = append(found, d)
		}
		require.NoError(t, seq.Err())

		// no order is promised: each way a workload runs is found by what it is and how it runs.
		ways := make(map[string]workspaces.Detected, len(found))
		for _, d := range found {
			ways[d.Workload+"|"+string(d.Mode)] = d
		}
		require.Len(t, found, 5)
		require.Contains(t, ways, "|local")
		require.NotContains(t, ways, "helper|local", "only main packages are workloads")
		require.False(t, ways["b|serve"].Ignored)
		require.True(t, ways["ignored|local"].Ignored, "ignored workloads are reported, marked")
		require.Equal(t, []string{"--gcp-auto"}, ways["a|local"].Args)
		require.Equal(t, []string{"--ttl=infinity", "--secret=x-me"}, ways["b|serve"].Args)
		require.NotContains(t, ways, "b|local")
		require.Equal(t, filepath.Join(root, ".eg", "a"), ways["a|local"].Dir)
		require.Equal(t, []string{"compute", "local", "--directory", root, "--gcp-auto", "a"}, ways["a|local"].Command)
		require.Equal(t, []string{"compute", "upload", "--directory", root, "a"}, ways["a|upload"].Command)
		require.Equal(t, []string{"compute", "serve", "--directory", root, "--ssh", "--ttl=infinity", "--secret=x-me", "b"}, ways["b|serve"].Command)
	})

	t.Run("detect workloads without a module directory", func(t *testing.T) {
		seq := workspaces.DetectWorkloads(t.TempDir(), func(string) string { return "" })
		var found []workspaces.Detected
		for d := range seq.Each(t.Context()) {
			found = append(found, d)
		}
		require.NoError(t, seq.Err())
		require.Empty(t, found)
	})
}
