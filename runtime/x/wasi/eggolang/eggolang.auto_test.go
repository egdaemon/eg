package eggolang

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func testmodule(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, d, "go.mod"), []byte("module example.com/"+d+"\n"), 0600))
	}
	return root
}

func recorder() (shell.Command, *shell.Recorder) {
	base := shell.NewLocal()
	return base, shell.NewRecorder(&base)
}

// failing creates a runtime whose commands fail when the command line contains substr.
func failing(substr string) shell.Command {
	return shell.NewLocal().UnsafeExec(func(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
		if strings.Contains(strings.Join(args, " "), substr) {
			return errors.New("boom")
		}

		return nil
	})
}

func TestAuto(t *testing.T) {
	t.Run("AutoInstall runs go install per module", func(t *testing.T) {
		root := testmodule(t, "a", "b")
		base, rec := recorder()

		err := New().WorkingDirectory(root).Tags("integration").Debug(true).autoinstall(base)(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, rec.Results(), 2)
		require.Contains(t, rec.Results()[0], "go -C "+filepath.Join(root, "a")+" install -modcacherw -x -tags=integration ./...")
		require.Contains(t, rec.Results()[1], "go -C "+filepath.Join(root, "b")+" install -modcacherw -x -tags=integration ./...")
	})

	t.Run("AutoInstall without modules does nothing", func(t *testing.T) {
		base, rec := recorder()
		require.NoError(t, New().WorkingDirectory(t.TempDir()).autoinstall(base)(t.Context(), nil))
		require.Empty(t, rec.Results())
	})

	t.Run("AutoCompile runs go build per module", func(t *testing.T) {
		root := testmodule(t, "a")
		base, rec := recorder()

		require.NoError(t, New().WorkingDirectory(root).Environ("FOO=bar").autocompile(base)(t.Context(), nil))
		require.Len(t, rec.Results(), 1)
		require.Contains(t, rec.Results()[0], "go -C "+filepath.Join(root, "a")+" build -modcacherw")
		require.Contains(t, rec.Results()[0], "FOO=bar")
	})

	t.Run("AutoTest creates the coverage directory and runs go test per module", func(t *testing.T) {
		root := testmodule(t, "a")
		base, rec := recorder()

		testx.Tempenvvar(_eg.EnvComputeWorkingDirectory, root, func() {
			require.NoError(t, New().Count(2).Verbose(true).autotest(base)(t.Context(), nil))
		})

		require.Len(t, rec.Results(), 2)
		require.Contains(t, rec.Results()[0], "mkdir -p "+coveragedir())
		require.Contains(t, rec.Results()[1], "go -C "+filepath.Join(root, "a")+" test -modcacherw")
		require.Contains(t, rec.Results()[1], "-count 2 -v -covermode count")
		require.Contains(t, rec.Results()[1], "-coverprofile "+coveragedir())
	})

	t.Run("failures are reported", func(t *testing.T) {
		root := testmodule(t, "a")

		require.ErrorContains(t, New().WorkingDirectory(root).autoinstall(failing(" install "))(t.Context(), nil), "unable to run tests")
		require.ErrorContains(t, New().WorkingDirectory(root).autocompile(failing(" build "))(t.Context(), nil), "unable to compile")

		testx.Tempenvvar(_eg.EnvComputeWorkingDirectory, root, func() {
			require.ErrorContains(t, New().autotest(failing("mkdir"))(t.Context(), nil), "unable to run tests")
			require.ErrorContains(t, New().autotest(failing(" test "))(t.Context(), nil), "unable to run tests")
		})
	})

	t.Run("wrappers build operations", func(t *testing.T) {
		require.NotNil(t, AutoInstall())
		require.NotNil(t, AutoCompile())
		require.NotNil(t, AutoTest())
	})
}
