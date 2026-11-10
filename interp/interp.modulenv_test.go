package interp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestRunnerModuleEnv(t *testing.T) {
	wasm := interptest.Build(t, "./interp/.xfixtures/environ")

	// the environment file both compute local and compute baremetal write to the runtime directory: the rest of the
	// environment, and the directories of the container.
	fixture := testx.XFixture("environ", "example.1.env")

	for _, tc := range []struct {
		name string
		// the workspace of the host, its runtime directory is replaced by a temporary directory holding the environment file.
		ws workspaces.Context
		// the environment of the interpreter.
		environ func(ws workspaces.Context) []string
		// the complete environment of the guest.
		expected func(ws workspaces.Context) []string
	}{
		{
			// the interpreter runs within the container, with the environment cmd/eg/runner.go builds for it.
			name: "compute local module environment",
			ws:   workspaces.Context{},
			environ: func(ws workspaces.Context) []string {
				return errorsx.Must(envx.Build().
					FromEnviron("FROM_HOST=1").
					Var(eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory()).
					Var(eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory()).
					Var(eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory()).
					Var(eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory()).
					Environ())
			},
			// the order is that of the environment the guest is given: the environment file, then the environment of the interpreter,
			// which wins. a key keeps the position it was first given, with the value of the last.
			expected: func(ws workspaces.Context) []string {
				return errorsx.Must(envx.Build().
					FromPath(testx.XFixture("environ", "example.local.env")).
					Environ())
			},
		},
		{
			// regression: before 2b3fd7be the environment file was read from the default mount root, which doesn't exist
			// on a baremetal host, so the directories of the host won. reading it from the runtime directory of the
			// workspace let its /workload defaults override them and any command with a working directory under
			// egenv.DirectoryWorkload() failed with "chdir /workload/.eg.workspace: no such file or directory".
			// baremetal passes the directories of the host as the environment of the interpreter, they must win.
			name: "compute baremetal module environment",
			ws: workspaces.Context{
				Root:         "/w/.eg.workload/m.bbbbbbbb.d",
				CacheDir:     "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache",
				WorkspaceDir: "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace",
				WorkingDir:   "/w/.eg.workload/m.bbbbbbbb.d/eg",
			},
			environ: func(ws workspaces.Context) []string {
				return errorsx.Must(envx.Build().
					FromEnviron("FROM_HOST=1", "SHARED=host").
					Var(eg.EnvComputeWorkloadDirectory, ws.Root).
					Var(eg.EnvComputeRuntimeDirectory, ws.RuntimeDir).
					Var(eg.EnvComputeCacheDirectory, ws.CacheDir).
					Var(eg.EnvComputeWorkspaceDirectory, ws.WorkspaceDir).
					Var(eg.EnvComputeWorkingDirectory, ws.WorkingDir).
					Environ())
			},
			// the environment file first, then the environment of the interpreter, which wins. the runtime directory is the
			// temporary directory holding the environment file, the example has a placeholder at its position.
			expected: func(ws workspaces.Context) []string {
				return errorsx.Must(envx.Build().
					FromReader(strings.NewReader(strings.ReplaceAll(testx.ReadString(testx.XFixture("environ", "example.baremetal.env")), "{runtime}", ws.RuntimeDir))).
					Environ())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := tc.ws
			ws.RuntimeDir = t.TempDir()
			require.NoError(t, envx.Build().FromPath(fixture).WriteTo(filepath.Join(ws.RuntimeDir, eg.EnvironFile)))

			r := runner{environ: tc.environ(ws)}

			ctx, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()

			rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
			defer rt.Close(context.Background())

			_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
			require.NoError(t, err)

			bin, err := os.ReadFile(wasm)
			require.NoError(t, err)

			var stdout bytes.Buffer
			_, err = rt.InstantiateWithConfig(ctx, bin, r.modulenv(wazero.NewModuleConfig().WithStdout(&stdout).WithStderr(os.Stderr), ws))
			require.NoError(t, err)

			// the guest prints its environment one entry per line, in the order it was given.
			require.Equal(t, strings.Join(tc.expected(ws), "\n")+"\n", stdout.String())
		})
	}
}
