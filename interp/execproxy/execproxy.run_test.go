package execproxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRun(t *testing.T) {
	client := func(t *testing.T, root string, environ ...string) ProxyClient {
		// unix socket paths are length limited, t.TempDir can exceed it.
		dir, err := os.MkdirTemp("", "execproxy")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dir) })
		socket := filepath.Join(dir, "exec.sock")
		l, err := net.Listen("unix", socket)
		require.NoError(t, err)
		srv := grpc.NewServer()
		NewExecProxy(root, environ).Bind(srv)
		go srv.Serve(l)
		t.Cleanup(srv.Stop)

		cc, err := grpc.NewClient("unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { cc.Close() })
		return NewProxyClient(cc)
	}

	run := func(t *testing.T, c ProxyClient, stdin string, cmd string, args ...string) (int32, string, string, error) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)
		code, err := Run(t.Context(), c, &ExecRequest{Cmd: cmd, Arguments: args}, strings.NewReader(stdin), &stdout, &stderr)
		return code, stdout.String(), stderr.String(), err
	}

	t.Run("captures stdout and stderr separately", func(t *testing.T) {
		code, stdout, stderr, err := run(t, client(t, t.TempDir()), "", "sh", "-c", "echo out; echo err >&2")
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Equal(t, "out\n", stdout)
		require.Equal(t, "err\n", stderr)
	})

	t.Run("returns the exit code", func(t *testing.T) {
		code, _, _, err := run(t, client(t, t.TempDir()), "", "sh", "-c", "exit 3")
		require.NoError(t, err)
		require.Equal(t, int32(3), code)
	})

	t.Run("reports a signal as 128 plus the signal", func(t *testing.T) {
		code, _, _, err := run(t, client(t, t.TempDir()), "", "sh", "-c", "kill -9 $$")
		require.NoError(t, err)
		require.Equal(t, int32(137), code)
	})

	t.Run("copies stdin to the command", func(t *testing.T) {
		code, stdout, _, err := run(t, client(t, t.TempDir()), "pear\napple\n", "sort")
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Equal(t, "apple\npear\n", stdout)
	})

	t.Run("nil stdin closes the command's stdin", func(t *testing.T) {
		var stdout bytes.Buffer
		code, err := Run(t.Context(), client(t, t.TempDir()), &ExecRequest{Cmd: "cat"}, nil, &stdout, &bytes.Buffer{})
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Empty(t, stdout.String())
	})

	t.Run("streams more than the pipe buffers in both directions", func(t *testing.T) {
		var stdout bytes.Buffer
		data := make([]byte, 4<<20)
		_, err := rand.Read(data)
		require.NoError(t, err)
		code, err := Run(t.Context(), client(t, t.TempDir()), &ExecRequest{Cmd: "cat"}, bytes.NewReader(data), &stdout, &bytes.Buffer{})
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Equal(t, data, stdout.Bytes())
	})

	t.Run("resolves relative directories against the root", func(t *testing.T) {
		root := t.TempDir()
		var stdout bytes.Buffer
		code, err := Run(t.Context(), client(t, root), &ExecRequest{Cmd: "pwd", Dir: "."}, nil, &stdout, &bytes.Buffer{})
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Equal(t, root+"\n", stdout.String())
	})

	t.Run("passes the proxy and request environment", func(t *testing.T) {
		var stdout bytes.Buffer
		code, err := Run(t.Context(), client(t, t.TempDir(), "A=proxy"), &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo $A $B"}, Environment: []string{"B=request"}}, nil, &stdout, &bytes.Buffer{})
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Equal(t, "proxy request\n", stdout.String())
	})

	t.Run("missing binary", func(t *testing.T) {
		_, _, _, err := run(t, client(t, t.TempDir()), "", "eg-does-not-exist")
		require.Error(t, err)
	})

	t.Run("cancellation kills the command", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer done()
		ts := time.Now()
		_, err := Run(ctx, client(t, t.TempDir()), &ExecRequest{Cmd: "sleep", Arguments: []string{"30"}}, nil, &bytes.Buffer{}, &bytes.Buffer{})
		require.Error(t, err)
		require.Less(t, time.Since(ts), 5*time.Second)
	})
}
