package ffiwasi_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a wasip1 guest")
	}

	guest := interptest.Build(t, "./interp/runtime/wasi/ffiwasi/.fixtures/guest")
	module := interptest.Build(t, "./interp/runtime/wasi/ffiwasi/.fixtures/module")

	// run runs the guest in mode with the module fixture linked into the module set as tee, env and code. the
	// runtime directory has no module socket, so a command only succeeds if it runs in the runtime.
	run := func(t *testing.T, mode string) error {
		moddir := t.TempDir()
		for _, name := range []string{"tee", "env", "code"} {
			require.NoError(t, os.Symlink(module, filepath.Join(moddir, name+".wasi")))
		}

		// unix socket paths are length limited, t.TempDir can exceed it.
		rundir, err := os.MkdirTemp("", "ffiwasi")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(rundir) })
		require.NoError(t, os.Mkdir(filepath.Join(rundir, "work"), 0700))

		return interptest.RunModules(t, guest, rundir, moddir, filepath.Join(rundir, "work"), time.Minute, "WASI_MODE="+mode, eg.EnvComputeWorkingDirectory+"="+eg.DefaultRuntimeDirectory("work"))
	}

	for _, mode := range []string{"tee", "env", "code", "notfound"} {
		t.Run(mode, func(t *testing.T) {
			require.NoError(t, run(t, mode))
		})
	}
}
