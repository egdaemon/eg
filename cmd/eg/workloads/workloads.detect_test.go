package workloads

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdtestx/v2"
	egcompute "github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/jsonx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/stretchr/testify/require"
)

func TestDetect(t *testing.T) {
	t.Run("detects the workloads of the repositories in the order given", func(t *testing.T) {
		first, second := t.TempDir(), t.TempDir()

		for _, root := range []string{first, second} {
			for rel, content := range map[string]string{
				"go.mod":               "module fixture\n\ngo 1.24\n",
				".eg/main.go":          "package main\n\nfunc main() {}\n",
				".eg/a/main.go":        "package main\n\nfunc main() {}\n",
				".eg/a/.egrc":          "[local]\n--gcp-auto\n[serve]\n--secret=x-${DETECT_TEST_USER}\n",
				".eg/skip/main.go":     "package main\n\nfunc main() {}\n",
				".eg/skip/.egrcignore": "",
			} {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
				require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0600))
			}
		}

		var stdout bytes.Buffer
		harness := cmdtestx.New(t, &detectCmd{}).Env("DETECT_TEST_USER=me").Options(kong.Writers(&stdout, &stdout), kong.Vars{"vars_eg_root_directory": "."})
		require.NoError(t, harness.Run(first, t.TempDir(), second, "--jsonl"))

		var found []*egcompute.Workload
		for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
			w := &egcompute.Workload{}
			require.NoError(t, jsonx.Unmarshal(line, w))
			found = append(found, w)
		}
		require.Len(t, found, 8)

		// no order is promised: each way a workload runs is found by where it is, what it is and how it runs.
		ways := make(map[string]*egcompute.Workload, len(found))
		for _, w := range found {
			ways[w.Directory+"|"+w.Workload+"|"+w.Command[1]] = w
		}

		// the default workload only runs local, a runs local and serve.
		require.Equal(t, []string{"compute", "local", "--directory", first}, ways[first+"||local"].Command)
		require.Equal(t, []string{"compute", "local", "--directory", first, "--gcp-auto", "a"}, ways[first+"|a|local"].Command)
		require.Equal(t, []string{"compute", "serve", "--directory", first, "--ssh", "--secret=x-me", "a"}, ways[first+"|a|serve"].Command)
		require.NotContains(t, ways, first+"|a|upload")
		// the ignored workload is reported, marked, for tools to decide whether to show it.
		require.True(t, ways[first+"|skip|local"].Ignored)
		require.False(t, ways[first+"|a|serve"].Ignored)
		require.Contains(t, ways, second+"||local")
	})

	t.Run("lists ignored workloads in the table as disabled", func(t *testing.T) {
		root := t.TempDir()
		for rel, content := range map[string]string{
			"go.mod":               "module fixture\n\ngo 1.24\n",
			".eg/main.go":          "package main\n\nfunc main() {}\n",
			".eg/skip/main.go":     "package main\n\nfunc main() {}\n",
			".eg/skip/.egrcignore": "",
		} {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0600))
		}

		var table bytes.Buffer
		harness := cmdtestx.New(t, &detectCmd{}).Options(kong.Writers(&table, &table), kong.Vars{"vars_eg_root_directory": "."})
		require.NoError(t, harness.Run(root))
		require.Contains(t, table.String(), "WORKLOAD")
		require.Regexp(t, `\(default\)\s+enabled`, table.String())
		require.Regexp(t, `skip\s+disabled`, table.String())
	})

	t.Run("reports the canonical vcs uri of the repository, blank without an origin", func(t *testing.T) {
		withorigin, without := t.TempDir(), t.TempDir()
		for _, root := range []string{withorigin, without} {
			require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0600))
			require.NoError(t, os.MkdirAll(filepath.Join(root, ".eg"), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, ".eg", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0600))
		}

		repo, err := git.PlainInit(withorigin, false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&config.RemoteConfig{Name: git.DefaultRemoteName, URLs: []string{"https://github.com/egdaemon/eg"}})
		require.NoError(t, err)

		var stdout bytes.Buffer
		harness := cmdtestx.New(t, &detectCmd{}).Options(kong.Writers(&stdout, &stdout), kong.Vars{"vars_eg_root_directory": "."})
		require.NoError(t, harness.Run(withorigin, without, "--jsonl"))

		var found []*egcompute.Workload
		for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
			w := &egcompute.Workload{}
			require.NoError(t, jsonx.Unmarshal(line, w))
			found = append(found, w)
		}
		require.Len(t, found, 2)
		require.Equal(t, "git@github.com:egdaemon/eg", found[0].Vcsuri)
		require.Equal(t, "", found[1].Vcsuri)
	})

	t.Run("a failing repository does not hide the others", func(t *testing.T) {
		good := t.TempDir()
		// an unreadable egrc fails its repository.
		bad := t.TempDir()
		for _, root := range []string{good, bad} {
			require.NoError(t, os.MkdirAll(filepath.Join(root, ".eg"), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(root, ".eg", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0600))
		}
		require.NoError(t, os.Mkdir(filepath.Join(bad, ".eg", ".egrc"), 0700))

		// the workloads of the good repository are all there, the failure is the error.
		var stdout bytes.Buffer
		harness := cmdtestx.New(t, &detectCmd{}).Options(kong.Writers(&stdout, &stdout), kong.Vars{"vars_eg_root_directory": "."})
		require.Error(t, harness.Run(bad, good, "--jsonl"))

		var found []*egcompute.Workload
		for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
			w := &egcompute.Workload{}
			require.NoError(t, jsonx.Unmarshal(line, w))
			found = append(found, w)
		}
		require.Len(t, found, 1)
		require.Equal(t, good, found[0].Directory)

		var failing bytes.Buffer
		harness = cmdtestx.New(t, &detectCmd{}).Options(kong.Writers(&failing, &failing), kong.Vars{"vars_eg_root_directory": "."})
		require.Error(t, harness.Run(bad, "--jsonl"))
		require.Empty(t, failing.String())
	})

	t.Run("writes json lines and a table", func(t *testing.T) {
		root := t.TempDir()
		for rel, content := range map[string]string{
			"go.mod":        "module fixture\n\ngo 1.24\n",
			".eg/main.go":   "package main\n\nfunc main() {}\n",
			".eg/a/main.go": "package main\n\nfunc main() {}\n",
			".eg/a/.egrc":   "[serve]\n--secret=it's here\n",
		} {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0600))
		}

		var jsonl bytes.Buffer
		harness := cmdtestx.New(t, &detectCmd{}).Options(kong.Writers(&jsonl, &jsonl), kong.Vars{"vars_eg_root_directory": "."})
		require.NoError(t, harness.Run(root, "--jsonl"))
		require.ElementsMatch(t, []string{
			`{"directory":"` + root + `","command":["compute","local","--directory","` + root + `"]}`,
			`{"workload":"a","directory":"` + root + `","command":["compute","serve","--directory","` + root + `","--ssh","--secret=it's here","a"]}`,
		}, strings.Split(strings.TrimSpace(jsonl.String()), "\n"))

		var table bytes.Buffer
		harness = cmdtestx.New(t, &detectCmd{}).Options(kong.Writers(&table, &table), kong.Vars{"vars_eg_root_directory": "."})
		require.NoError(t, harness.Run(root))
		require.Contains(t, table.String(), "WORKLOAD")
		require.Contains(t, table.String(), "(default)")
		require.Contains(t, table.String(), "--secret=it's here")
	})
}
