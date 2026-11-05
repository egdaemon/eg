package execproxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/langx"
	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// serve binds the proxy rooted at root to a control socket in a new runtime directory, and returns the directory.
func serve(t *testing.T, root string, environ ...string) string {
	// unix socket paths are length limited, t.TempDir can exceed it.
	runtimedir, err := os.MkdirTemp("", "execproxy")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(runtimedir) })

	l, err := net.Listen("unix", filepath.Join(runtimedir, eg.SocketControl))
	require.NoError(t, err)
	srv := grpc.NewServer()
	NewProxy(root, runtimedir, environ, wazero.NewFSConfig()).Bind(srv)
	go srv.Serve(l)
	t.Cleanup(srv.Stop)
	return runtimedir
}

func dial(t *testing.T, runtimedir string) v1.ProxyClient {
	cc, err := grpc.NewClient("unix://"+filepath.Join(runtimedir, eg.SocketControl), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { cc.Close() })
	return v1.NewProxyClient(cc)
}

func TestRun(t *testing.T) {
	type result struct {
		code   int32
		stdout string
		stderr string
	}

	listen := func(path string) (net.Listener, error) {
		return net.Listen("unix", path)
	}

	run := func(ctx context.Context, t *testing.T, runtimedir string, stdin io.Reader, req *v1.ExecRequest) (result, error) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)
		code, err := Run(ctx, dial(t, runtimedir), listen, runtimedir, req, stdin, &stdout, &stderr)
		return result{code: code, stdout: stdout.String(), stderr: stderr.String()}, err
	}

	t.Run("captures stdout and stderr separately", func(t *testing.T) {
		r, err := run(t.Context(), t, serve(t, t.TempDir()), nil, &v1.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo out; echo err >&2"}})
		require.NoError(t, err)
		require.Equal(t, result{code: 0, stdout: "out\n", stderr: "err\n"}, r)
	})

	t.Run("returns the exit code", func(t *testing.T) {
		r, err := run(t.Context(), t, serve(t, t.TempDir()), nil, &v1.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "exit 3"}})
		require.NoError(t, err)
		require.Equal(t, int32(3), r.code)
	})

	t.Run("reports a signal as 128 plus the signal", func(t *testing.T) {
		r, err := run(t.Context(), t, serve(t, t.TempDir()), nil, &v1.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "kill -9 $$"}})
		require.NoError(t, err)
		require.Equal(t, int32(137), r.code)
	})

	t.Run("copies stdin to the command", func(t *testing.T) {
		r, err := run(t.Context(), t, serve(t, t.TempDir()), strings.NewReader("pear\napple\n"), &v1.ExecRequest{Cmd: "sort"})
		require.NoError(t, err)
		require.Equal(t, "apple\npear\n", r.stdout)
	})

	t.Run("nil stdin ends the command's input", func(t *testing.T) {
		r, err := run(t.Context(), t, serve(t, t.TempDir()), nil, &v1.ExecRequest{Cmd: "cat"})
		require.NoError(t, err)
		require.Equal(t, result{}, r)
	})

	t.Run("streams more than the socket buffers in both directions", func(t *testing.T) {
		data := make([]byte, 4<<20)
		_, err := rand.Read(data)
		require.NoError(t, err)
		r, err := run(t.Context(), t, serve(t, t.TempDir()), bytes.NewReader(data), &v1.ExecRequest{Cmd: "cat"})
		require.NoError(t, err)
		require.Equal(t, int32(0), r.code)
		require.Equal(t, data, []byte(r.stdout))
	})

	t.Run("resolves relative directories against the root", func(t *testing.T) {
		root := t.TempDir()
		r, err := run(t.Context(), t, serve(t, root), nil, &v1.ExecRequest{Cmd: "pwd", Dir: "."})
		require.NoError(t, err)
		require.Equal(t, root+"\n", r.stdout)
	})

	t.Run("passes the proxy and request environment", func(t *testing.T) {
		r, err := run(t.Context(), t, serve(t, t.TempDir(), "A=proxy"), nil, &v1.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo $A $B"}, Environment: []string{"B=request"}})
		require.NoError(t, err)
		require.Equal(t, "proxy request\n", r.stdout)
	})

	t.Run("missing binary", func(t *testing.T) {
		_, err := run(t.Context(), t, serve(t, t.TempDir()), nil, &v1.ExecRequest{Cmd: "eg-does-not-exist"})
		require.Error(t, err)
	})

	t.Run("cancellation kills the command", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer done()
		ts := time.Now()
		_, err := run(ctx, t, serve(t, t.TempDir()), nil, &v1.ExecRequest{Cmd: "sleep", Arguments: []string{"30"}})
		require.Error(t, err)
		require.Less(t, time.Since(ts), 5*time.Second)
	})

	t.Run("removes the process directory", func(t *testing.T) {
		runtimedir := serve(t, t.TempDir())
		_, err := run(t.Context(), t, runtimedir, nil, &v1.ExecRequest{Cmd: "true"})
		require.NoError(t, err)
		entries, err := os.ReadDir(filepath.Join(runtimedir, "proc"))
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("wasip1 guest", func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds and runs a wasip1 guest")
		}

		wasm := interptest.Build(t, "./interp/execproxy/v2/.fixtures/stream")
		for _, mode := range []string{"output", "exit", "stdin", "pipeline"} {
			t.Run(mode, func(t *testing.T) {
				require.NoError(t, interptest.Run(t, wasm, serve(t, t.TempDir(), os.Environ()...), time.Minute, "STREAM_MODE="+mode))
			})
		}
	})
}

func TestExec(t *testing.T) {
	t.Run("rejects an id outside of proc", func(t *testing.T) {
		c := dial(t, serve(t, t.TempDir()))
		for _, id := range []string{"../escape", "a/b", "..", "/abs"} {
			_, err := c.Exec(t.Context(), &v1.ExecRequest{Id: id, Cmd: "true"})
			require.ErrorContains(t, err, "invalid process id", id)
		}
	})

	t.Run("missing stdio sockets", func(t *testing.T) {
		c := dial(t, serve(t, t.TempDir()))
		_, err := c.Exec(t.Context(), &v1.ExecRequest{Id: "missing", Cmd: "true"})
		require.ErrorContains(t, err, "unable to connect the stdio of process missing")
	})

	t.Run("without an id a failed exit is an error", func(t *testing.T) {
		c := dial(t, serve(t, t.TempDir()))
		_, err := c.Exec(t.Context(), &v1.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "exit 3"}})
		require.Error(t, err)
		_, err = c.Exec(t.Context(), &v1.ExecRequest{Cmd: "true"})
		require.NoError(t, err)
	})
}

func TestAuto(t *testing.T) {
	t.Run("defaults to v1", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalProcSockets, "")
		require.IsType(t, &v1.ExecProxy{}, Auto(t.TempDir(), t.TempDir(), nil, wazero.NewFSConfig()))
	})

	t.Run("experiment selects v2", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalProcSockets, "true")
		require.IsType(t, &Proxy{}, Auto(t.TempDir(), t.TempDir(), nil, wazero.NewFSConfig()))
	})
}

func TestWasm(t *testing.T) {
	type result struct {
		code   int32
		stdout string
	}

	wasm := interptest.Build(t, "./interp/execproxy/v2/.fixtures/wasm")

	// serve binds a proxy whose coreutils module is the fixture at path, with mounts as the guest's.
	serve := func(t *testing.T, root string, mounts wazero.FSConfig, path string, environ ...string) string {
		runtimedir, err := os.MkdirTemp("", "execproxy")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(runtimedir) })

		l, err := net.Listen("unix", filepath.Join(runtimedir, eg.SocketControl))
		require.NoError(t, err)
		p := NewProxy(root, runtimedir, environ, mounts)
		p.wasms["coreutils"] = compiler(p.runtime, path)
		srv := grpc.NewServer()
		p.Bind(srv)
		go srv.Serve(l)
		t.Cleanup(srv.Stop)
		return runtimedir
	}

	run := func(t *testing.T, runtimedir string, stdin io.Reader, req *v1.ExecRequest) (result, error) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)
		listen := func(path string) (net.Listener, error) {
			return net.Listen("unix", path)
		}
		req.Wasm = langx.FirstNonZero(req.Wasm, "coreutils")
		code, err := Run(t.Context(), dial(t, runtimedir), listen, runtimedir, req, stdin, &stdout, &stderr)
		return result{code: code, stdout: stdout.String()}, err
	}

	t.Run("connects the module to the command's stdio", func(t *testing.T) {
		r, err := run(t, serve(t, t.TempDir(), wazero.NewFSConfig(), wasm), strings.NewReader("hello"), &v1.ExecRequest{Cmd: "cat"})
		require.NoError(t, err)
		require.Equal(t, result{stdout: "hello"}, r)
	})

	t.Run("resolves relative paths against the command's directory", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "file"), []byte("relative"), 0600))
		r, err := run(t, serve(t, root, wazero.NewFSConfig(), wasm), nil, &v1.ExecRequest{Cmd: "cat", Arguments: []string{"file"}, Dir: "sub"})
		require.NoError(t, err)
		require.Equal(t, result{stdout: "relative"}, r)
	})

	t.Run("writes relative paths into the command's directory", func(t *testing.T) {
		root := t.TempDir()
		_, err := run(t, serve(t, root, wazero.NewFSConfig(), wasm), nil, &v1.ExecRequest{Cmd: "write", Arguments: []string{"file", "written"}, Dir: "."})
		require.NoError(t, err)
		b, err := os.ReadFile(filepath.Join(root, "file"))
		require.NoError(t, err)
		require.Equal(t, "written", string(b))
	})

	t.Run("reaches the guest's mounts", func(t *testing.T) {
		mounted := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(mounted, "file"), []byte("mounted"), 0600))
		r, err := run(t, serve(t, t.TempDir(), wazero.NewFSConfig().WithDirMount(mounted, "/workload"), wasm), nil, &v1.ExecRequest{Cmd: "cat", Arguments: []string{"/workload/file"}})
		require.NoError(t, err)
		require.Equal(t, result{stdout: "mounted"}, r)
	})

	t.Run("returns the module's exit code", func(t *testing.T) {
		r, err := run(t, serve(t, t.TempDir(), wazero.NewFSConfig(), wasm), nil, &v1.ExecRequest{Cmd: "exit", Arguments: []string{"3"}})
		require.NoError(t, err)
		require.Equal(t, int32(3), r.code)
	})

	t.Run("passes the proxy and request environment", func(t *testing.T) {
		runtimedir := serve(t, t.TempDir(), wazero.NewFSConfig(), wasm, "A=proxy")
		r, err := run(t, runtimedir, nil, &v1.ExecRequest{Cmd: "env", Arguments: []string{"A"}})
		require.NoError(t, err)
		require.Equal(t, "proxy\n", r.stdout)
		r, err = run(t, runtimedir, nil, &v1.ExecRequest{Cmd: "env", Arguments: []string{"B"}, Environment: []string{"B=request"}})
		require.NoError(t, err)
		require.Equal(t, "request\n", r.stdout)
	})

	t.Run("runs natively when the module is not installed", func(t *testing.T) {
		runtimedir := serve(t, t.TempDir(), wazero.NewFSConfig(), filepath.Join(t.TempDir(), "missing.wasm"))
		r, err := run(t, runtimedir, nil, &v1.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo native"}})
		require.NoError(t, err)
		require.Equal(t, result{stdout: "native\n"}, r)
	})

	t.Run("rejects an unknown module", func(t *testing.T) {
		_, err := run(t, serve(t, t.TempDir(), wazero.NewFSConfig(), wasm), nil, &v1.ExecRequest{Cmd: "cat", Wasm: "unknown"})
		require.Error(t, err)
	})
}
