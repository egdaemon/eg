package ffiegvm_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegvm"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/protobuf/proto"
)

func TestProcs(t *testing.T) {
	wasm := interptest.Build(t, "./interp/runtime/wasi/ffiegvm/.xfixtures/exec")

	// run runs the guest with exec behind the machine, returning its stdout and stderr.
	run := func(t *testing.T, stdin string, exec ffiegvm.Exec) (string, string) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)

		// unix socket paths are limited to about 100 bytes, more than test directories leave.
		runtimedir, err := os.MkdirTemp("/tmp", "procs")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(runtimedir) })

		ctx := t.Context()
		rt := wazero.NewRuntime(ctx)
		defer rt.Close(context.Background())
		wasi_snapshot_preview1.MustInstantiate(ctx, rt)

		_, err = ffiwasinet.Wazero(
			rt,
			wnetruntime.OptionFSPrefixes(wnetruntime.FSPrefix{Host: runtimedir, Guest: eg.DefaultRuntimeDirectory()}),
		).Instantiate(ctx)
		require.NoError(t, err)

		_, err = ffiegvm.NewProcs(ctx, runtimedir, exec).Export(rt.NewHostModuleBuilder("env")).Instantiate(ctx)
		require.NoError(t, err)

		bin, err := os.ReadFile(wasm)
		require.NoError(t, err)
		_, err = rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().
			WithFSConfig(wazero.NewFSConfig().WithDirMount(runtimedir, eg.DefaultRuntimeDirectory())).
			WithStdin(strings.NewReader(stdin)).
			WithStdout(&stdout).
			WithStderr(&stderr).
			WithOsyield(runtime.Gosched).
			WithSysNanosleep().
			WithSysNanotime().
			WithSysWalltime())
		require.NoError(t, err)

		return stdout.String(), stderr.String()
	}

	t.Run("runs the guest's command with its arguments, environment, directory and stdio", func(t *testing.T) {
		var (
			got *execproxy.ExecRequest
		)

		stdout, stderr := run(t, "input", func(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error) {
			got = req
			b, err := io.ReadAll(stdin)
			if err != nil {
				return -1, err
			}
			io.WriteString(stdout, "out:"+string(b))
			io.WriteString(stderr, "err")
			return 0, nil
		})
		require.Equal(t, "out:input", stdout)
		require.Equal(t, "err", stderr)
		require.True(t, proto.Equal(&execproxy.ExecRequest{Cmd: "tool", Arguments: []string{"a", "b"}, Environment: []string{"A=1"}, Dir: "/srv"}, got), got)
	})

	t.Run("reports the command's exit code", func(t *testing.T) {
		stdout, _ := run(t, "", func(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error) {
			return 3, nil
		})
		require.Contains(t, stdout, "tool exited with code 3")
	})

	t.Run("provides no commands without a machine", func(t *testing.T) {
		stdout, _ := run(t, "", nil)
		require.Contains(t, stdout, "the machine does not provide tool")
	})

	t.Run("reports a machine that fails to run the command", func(t *testing.T) {
		stdout, _ := run(t, "", func(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error) {
			return -1, errorsx.New("boom")
		})
		require.Contains(t, stdout, "unable to wait on tool")
	})
}
