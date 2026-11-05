// Package interptest provides helpers for tests that run a compiled guest in the wasi runtime,
// wired like the interpreter: wasi preview1, the wasinet host module, and an analytics server
// listening in the guest's runtime directory.
package interptest

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Build compiles the main package at pkg (relative to the module root, e.g. "./runtime/.../.fixtures/guest")
// for wasip1 and returns the path of the module. the test is skipped when it cannot be built.
func Build(t *testing.T, pkg string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	out := filepath.Join(t.TempDir(), "guest.wasm")
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = filepath.Dir(filepath.Dir(filepath.Dir(file)))
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("unable to build the wasip1 guest %s: %v\n%s", pkg, err, b)
	}

	return out
}

// Analytics is a running analytics server, as the runner provides to a workload.
type Analytics struct {
	// RuntimeDir is the host directory mounted as the guest's runtime directory.
	RuntimeDir string
	// DB is the database behind the server.
	DB *sql.DB
}

// StartAnalytics starts an analytics server, backed by a prepared database, listening on the
// analytics socket inside of a temporary runtime directory. it is stopped when the test ends.
func StartAnalytics(t *testing.T, opts ...duckproxyserver.Option) Analytics {
	t.Helper()

	rundir := t.TempDir()

	db, err := sql.Open("duckdb", filepath.Join(rundir, "analytics.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	pctx, pdone := context.WithTimeout(context.Background(), 10*time.Second)
	defer pdone()
	require.NoError(t, events.PrepareDB(pctx, db))

	sctx, stop := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- duckproxyserver.ListenUnix(sctx, filepath.Join(rundir, eg.SocketAnalytics), duckproxyserver.New(db, opts...))
	}()
	t.Cleanup(func() {
		stop()
		<-served
	})

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(rundir, eg.SocketAnalytics))
		return err == nil
	}, 2*time.Second, 10*time.Millisecond)

	return Analytics{RuntimeDir: rundir, DB: db}
}

// Run executes the module in the wasi runtime until it exits or the timeout elapses.
// the runtime directory is mounted at the guest's runtime directory and environ is
// set as KEY=VALUE pairs. a non zero exit is returned as an error.
func Run(t *testing.T, wasm string, rundir string, timeout time.Duration, environ ...string) error {
	t.Helper()

	ctx, done := context.WithTimeout(context.Background(), timeout)
	defer done()

	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer rt.Close(context.Background())

	_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
	require.NoError(t, err)

	_, err = ffiwasinet.Wazero(
		rt,
		wnetruntime.OptionFSPrefixes(wnetruntime.FSPrefix{Host: rundir, Guest: eg.DefaultRuntimeDirectory()}),
	).Instantiate(ctx)
	require.NoError(t, err)

	bin, err := os.ReadFile(wasm)
	require.NoError(t, err)

	compiled, err := rt.CompileModule(ctx, bin)
	require.NoError(t, err)

	cfg := wazero.NewModuleConfig().
		WithFSConfig(wazero.NewFSConfig().WithDirMount(rundir, eg.DefaultRuntimeDirectory())).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithSysNanotime().
		WithSysWalltime().
		WithName(filepath.Base(wasm))

	for _, kv := range environ {
		for i := range kv {
			if kv[i] == '=' {
				cfg = cfg.WithEnv(kv[:i], kv[i+1:])
				break
			}
		}
	}

	_, err = rt.InstantiateModule(ctx, compiled, cfg)
	return err
}
