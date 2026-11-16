package compute

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestServeContainerfile(t *testing.T) {
	t.Run("nearest containerfile wins, from the workload up to the module directory", func(t *testing.T) {
		root := t.TempDir()
		for _, rel := range []string{
			".eg/dev/Containerfile",
			".eg/dev/own/Containerfile",
			".eg/dev/own/deep/main.go",
			".eg/dev/shared/main.go",
			".eg/other/main.go",
			"Containerfile", // outside the module directory, never used.
		} {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("FROM scratch\n"), 0600))
		}

		for name, expected := range map[string]string{
			"dev/shared":   ".eg/dev/Containerfile",
			"dev/own":      ".eg/dev/own/Containerfile",
			"dev/own/deep": ".eg/dev/own/Containerfile",
			"dev":          ".eg/dev/Containerfile",
		} {
			found, err := serve{Dir: root, ModuleDir: ".eg", Name: name}.containerfile()
			require.NoError(t, err, name)
			require.Equal(t, filepath.Join(root, expected), found, name)
		}

		// nothing between the workload and the module directory.
		_, err := serve{Dir: root, ModuleDir: ".eg", Name: "other"}.containerfile()
		require.ErrorContains(t, err, "no Containerfile")
	})

	t.Run("the containerfile flag is relative to the repository directory", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".eg", "dev"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".eg", "dev", "Containerfile"), []byte("FROM scratch\n"), 0600))

		found, err := serve{Dir: root, ModuleDir: ".eg", Name: "elsewhere", Containerfile: ".eg/dev/Containerfile"}.containerfile()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".eg", "dev", "Containerfile"), found)

		_, err = serve{Dir: root, ModuleDir: ".eg", Name: "x", Containerfile: ".eg/missing"}.containerfile()
		require.Error(t, err)
	})

	// end to end, parses the command line the same as eg does and serves a workload that makes changes within its
	// container. requires podman, the network to build the workload's image, and the eg binary to mount into it.
	t.Run("the changes made by the workload are committed to the branch when the command completes", func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds a container image and serves a workload in it")
		}

		// nested as an unprivileged user podman cannot create the containers, see TestSpooledRun.
		if runners.Nested() {
			t.Skip("podman in podman as an unprivileged user is not supported")
		}

		if _, err := exec.LookPath("podman"); err != nil {
			t.Skip("podman is not installed")
		}

		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
		defer cancel()

		// the eg binary is mounted into the container to run the workload, the test binary can't do that.
		egbin := filepath.Join(t.TempDir(), "eg")
		raw, err := exec.CommandContext(ctx, "go", "build", "-o", egbin, "github.com/egdaemon/eg/cmd/eg").CombinedOutput()
		require.NoError(t, err, string(raw))
		original := os.Args[0]
		os.Args[0] = egbin
		defer func() { os.Args[0] = original }()

		// a repository of the fixture, its workload commits one file and leaves another uncommitted.
		repo := t.TempDir()
		require.NoError(t, os.CopyFS(repo, os.DirFS(testx.XFixture("compute.serve", "repo"))))
		for _, name := range []string{"go.mod", "go.sum"} {
			content, err := os.ReadFile(filepath.Join("..", "..", "..", ".eg", name))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(repo, ".eg", name), content, 0600))
		}

		for _, args := range [][]string{
			{"init", "-q", "-b", "main"},
			{"config", "user.name", "test"},
			{"config", "user.email", "test@example.com"},
			{"add", "."},
			{"commit", "-qm", "init"},
		} {
			raw, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
			require.NoError(t, err, string(raw))
		}

		var cli struct {
			cmdopts.Global
			cmdopts.ComputeSession
			Serve serve `cmd:""`
		}
		cli.Context = ctx

		parser, err := kong.New(
			&cli,
			kong.Name("eg"),
			cmdopts.RuntimeResources{Arch: "amd64", OS: "linux"}.KongVars(),
			kong.Vars{
				"env_eg_account":               "EG_ACCOUNT",
				"env_eg_profile":               "EG_PROFILE",
				"vars_eg_root_directory":       repo,
				"vars_workload_directory":      ".eg",
				"vars_git_default_remote_name": "origin",
				"vars_git_head_reference":      "HEAD",
				"vars_ssh_key_path":            filepath.Join(t.TempDir(), "id"),
			},
			kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
			kong.Bind(&cli.Global, new(cmdopts.HotswapPath), new(cmdopts.ComputeSession), &cmdopts.CLIDB{}),
		)
		require.NoError(t, err)

		kctx, err := parser.Parse([]string{"serve", "--directory", repo, "--session-branch", "eg/e2e", "persist"})
		require.NoError(t, err)
		require.NoError(t, kctx.Run())

		show := func(args ...string) string {
			raw, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
			require.NoError(t, err, string(raw))
			return strings.TrimSpace(string(raw))
		}

		t.Run("the commit made by the workload is on the branch with its author", func(t *testing.T) {
			require.Equal(t, "eg: apply workload changes\ncommitted by the workload", show("log", "--format=%s", "main..eg/e2e"))
			require.Equal(t, "Ada", show("log", "--format=%an", "-n1", "eg/e2e~1"))
			require.Equal(t, "two", show("show", "eg/e2e:tracked.txt"))
		})

		t.Run("the uncommitted changes of the workload are committed on top", func(t *testing.T) {
			require.Equal(t, "uncommitted", show("show", "eg/e2e:uncommitted.txt"))
		})

		t.Run("the checked out branch is untouched", func(t *testing.T) {
			require.Equal(t, "main", show("rev-parse", "--abbrev-ref", "HEAD"))
			require.Equal(t, "one", show("show", "main:tracked.txt"))
		})
	})
}
