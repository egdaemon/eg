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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestProcProxy(t *testing.T) {
	type result struct {
		code   int32
		stdout string
		stderr string
	}

	// client serves a proxy rooted at root and returns a client for it along with its runtime directory.
	client := func(t *testing.T, root string, environ ...string) (ProxyClient, string) {
		// unix socket paths are length limited, t.TempDir can exceed it.
		runtimedir, err := os.MkdirTemp("", "procproxy")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(runtimedir) })
		socket := filepath.Join(runtimedir, "exec.sock")
		l, err := net.Listen("unix", socket)
		require.NoError(t, err)
		srv := grpc.NewServer()
		NewProcProxy(root, runtimedir, environ).Bind(srv)
		go srv.Serve(l)
		t.Cleanup(srv.Stop)

		cc, err := grpc.NewClient("unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { cc.Close() })
		return NewProxyClient(cc), runtimedir
	}

	// run does what the guest does: listen on the stdio sockets of the process, then exec it.
	run := func(ctx context.Context, t *testing.T, c ProxyClient, runtimedir string, stdin io.Reader, req *ExecRequest) (result, error) {
		var (
			stdout    bytes.Buffer
			stderr    bytes.Buffer
			listeners []net.Listener
			resp      *ExecResponse
			exited    = make(chan error, 1)
		)

		req.Id = "p1"
		proc := filepath.Join(runtimedir, "proc", req.Id)
		require.NoError(t, os.MkdirAll(proc, 0700))
		for _, name := range []string{"stdin", "stdout", "stderr"} {
			l, err := net.Listen("unix", filepath.Join(proc, name))
			require.NoError(t, err)
			defer l.Close()
			listeners = append(listeners, l)
		}

		go func() {
			var cause error
			resp, cause = c.Exec(ctx, req)
			for _, l := range listeners {
				l.Close()
			}
			exited <- cause
		}()

		copied := make(chan struct{}, 2)
		for i, l := range listeners {
			conn, err := l.Accept()
			if err != nil {
				break
			}
			defer conn.Close()
			switch i {
			case 0:
				go func() {
					if stdin != nil {
						io.Copy(conn, stdin)
					}
					conn.(*net.UnixConn).CloseWrite()
				}()
			case 1:
				go func() { io.Copy(&stdout, conn); copied <- struct{}{} }()
			case 2:
				go func() { io.Copy(&stderr, conn); copied <- struct{}{} }()
			}
		}

		if err := <-exited; err != nil {
			return result{}, err
		}
		<-copied
		<-copied
		return result{code: resp.Exit, stdout: stdout.String(), stderr: stderr.String()}, nil
	}

	t.Run("captures stdout and stderr separately", func(t *testing.T) {
		c, rdir := client(t, t.TempDir())
		r, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo out; echo err >&2"}})
		require.NoError(t, err)
		require.Equal(t, result{code: 0, stdout: "out\n", stderr: "err\n"}, r)
	})

	t.Run("returns the exit code", func(t *testing.T) {
		c, rdir := client(t, t.TempDir())
		r, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "exit 3"}})
		require.NoError(t, err)
		require.Equal(t, int32(3), r.code)
	})

	t.Run("reports a signal as 128 plus the signal", func(t *testing.T) {
		c, rdir := client(t, t.TempDir())
		r, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "kill -9 $$"}})
		require.NoError(t, err)
		require.Equal(t, int32(137), r.code)
	})

	t.Run("copies stdin to the command", func(t *testing.T) {
		c, rdir := client(t, t.TempDir())
		r, err := run(t.Context(), t, c, rdir, strings.NewReader("pear\napple\n"), &ExecRequest{Cmd: "sort"})
		require.NoError(t, err)
		require.Equal(t, "apple\npear\n", r.stdout)
	})

	t.Run("closing stdin ends the command's input", func(t *testing.T) {
		c, rdir := client(t, t.TempDir())
		r, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "cat"})
		require.NoError(t, err)
		require.Equal(t, result{}, r)
	})

	t.Run("streams more than the socket buffers in both directions", func(t *testing.T) {
		data := make([]byte, 4<<20)
		_, err := rand.Read(data)
		require.NoError(t, err)
		c, rdir := client(t, t.TempDir())
		r, err := run(t.Context(), t, c, rdir, bytes.NewReader(data), &ExecRequest{Cmd: "cat"})
		require.NoError(t, err)
		require.Equal(t, int32(0), r.code)
		require.Equal(t, data, []byte(r.stdout))
	})

	t.Run("resolves relative directories against the root", func(t *testing.T) {
		root := t.TempDir()
		c, rdir := client(t, root)
		r, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "pwd", Dir: "."})
		require.NoError(t, err)
		require.Equal(t, root+"\n", r.stdout)
	})

	t.Run("passes the proxy and request environment", func(t *testing.T) {
		c, rdir := client(t, t.TempDir(), "A=proxy")
		r, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo $A $B"}, Environment: []string{"B=request"}})
		require.NoError(t, err)
		require.Equal(t, "proxy request\n", r.stdout)
	})

	t.Run("missing binary", func(t *testing.T) {
		c, rdir := client(t, t.TempDir())
		_, err := run(t.Context(), t, c, rdir, nil, &ExecRequest{Cmd: "eg-does-not-exist"})
		require.Error(t, err)
	})

	t.Run("cancellation kills the command", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer done()
		ts := time.Now()
		c, rdir := client(t, t.TempDir())
		_, err := run(ctx, t, c, rdir, nil, &ExecRequest{Cmd: "sleep", Arguments: []string{"30"}})
		require.Error(t, err)
		require.Less(t, time.Since(ts), 5*time.Second)
	})

	t.Run("rejects an id outside of proc", func(t *testing.T) {
		c, _ := client(t, t.TempDir())
		for _, id := range []string{"../escape", "a/b", "..", "/abs"} {
			_, err := c.Exec(t.Context(), &ExecRequest{Id: id, Cmd: "true"})
			require.ErrorContains(t, err, "invalid process id", id)
		}
	})

	t.Run("missing stdio sockets", func(t *testing.T) {
		c, _ := client(t, t.TempDir())
		_, err := c.Exec(t.Context(), &ExecRequest{Id: "missing", Cmd: "true"})
		require.ErrorContains(t, err, "unable to connect the stdio of process missing")
	})

	t.Run("without an id it runs as ExecProxy does", func(t *testing.T) {
		c, _ := client(t, t.TempDir())
		_, err := c.Exec(t.Context(), &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "exit 3"}})
		require.Error(t, err)
		_, err = c.Exec(t.Context(), &ExecRequest{Cmd: "true"})
		require.NoError(t, err)
	})
}

func TestAuto(t *testing.T) {
	t.Run("defaults to ExecProxy", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalProcSockets, "")
		require.IsType(t, &ExecProxy{}, Auto(t.TempDir(), t.TempDir(), nil))
	})

	t.Run("experiment selects ProcProxy", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalProcSockets, "true")
		require.IsType(t, &ProcProxy{}, Auto(t.TempDir(), t.TempDir(), nil))
	})
}
