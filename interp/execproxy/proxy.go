package execproxy

import (
	"bytes"
	context "context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

func NewExecProxy(root string, environ []string) *ExecProxy {
	return &ExecProxy{
		dir:     root,
		environ: environ,
	}
}

type ExecProxy struct {
	UnimplementedProxyServer
	dir     string
	environ []string
}

func (t *ExecProxy) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

// command prepares the request's command relative to the proxy's root directory and environment.
func (t *ExecProxy) command(ctx context.Context, req *ExecRequest) *exec.Cmd {
	cmd := exec.CommandContext(ctx, req.Cmd, req.Arguments...)
	cmd.Dir = req.Dir
	if !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(t.dir, cmd.Dir)
	}
	cmd.Env = append(t.environ, req.Environment...)
	return cmd
}

// Upload implements RunServer.
func (t *ExecProxy) Exec(ctx context.Context, req *ExecRequest) (resp *ExecResponse, err error) {
	var (
		cmd *exec.Cmd = t.command(ctx, req)
	)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = execx.MaybeRun(cmd); err != nil {
		return nil, err
	}

	return &ExecResponse{}, nil
}

// Stream implements ProxyServer, running the command of the first frame with its stdio carried by the stream.
func (t *ExecProxy) Stream(stream Proxy_StreamServer) (err error) {
	var (
		req    *StreamRequest
		cmd    *exec.Cmd
		stdin  io.WriteCloser
		stdout io.ReadCloser
		stderr io.ReadCloser
		mu     sync.Mutex
		wg     sync.WaitGroup
	)

	if req, err = stream.Recv(); err != nil {
		return err
	}

	cmd = t.command(stream.Context(), req.GetExec())
	if stdin, err = cmd.StdinPipe(); err != nil {
		return err
	}
	if stdout, err = cmd.StdoutPipe(); err != nil {
		return err
	}
	if stderr, err = cmd.StderrPipe(); err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}

	send := func(resp *StreamResponse) error {
		mu.Lock()
		defer mu.Unlock()
		return stream.Send(resp)
	}

	go func() {
		defer stdin.Close()
		for {
			frame, cause := stream.Recv()
			if cause != nil {
				return
			}
			if _, cause = stdin.Write(frame.GetStdin()); cause != nil {
				return
			}
		}
	}()

	// both pipes must be drained before Wait, which closes them.
	wg.Go(func() {
		pump(stdout, send, func(b []byte) *StreamResponse { return &StreamResponse{Body: &StreamResponse_Stdout{Stdout: b}} })
	})
	wg.Go(func() {
		pump(stderr, send, func(b []byte) *StreamResponse { return &StreamResponse{Body: &StreamResponse_Stderr{Stderr: b}} })
	})
	wg.Wait()

	if err = cmd.Wait(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return err
		}
	}

	return send(&StreamResponse{Body: &StreamResponse_Exit{Exit: exitcode(cmd.ProcessState)}})
}

// pump sends everything read from r as frames until r is exhausted or a send fails.
func pump(r io.Reader, send func(*StreamResponse) error, frame func([]byte) *StreamResponse) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if cause := send(frame(bytes.Clone(buf[:n]))); cause != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// exitcode reports a process killed by a signal as 128 plus the signal, like a shell does.
func exitcode(state *os.ProcessState) int32 {
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int32(ws.Signal())
	}
	return int32(state.ExitCode())
}

// Run executes req through c, copying stdin to the command and its output to stdout and stderr,
// and returns the command's exit code.
func Run(ctx context.Context, c ProxyClient, req *ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (code int32, err error) {
	var (
		stream Proxy_StreamClient
		resp   *StreamResponse
	)

	ctx, done := context.WithCancel(ctx)
	defer done()

	if stream, err = c.Stream(ctx); err != nil {
		return -1, err
	}
	if err = stream.Send(&StreamRequest{Body: &StreamRequest_Exec{Exec: req}}); err != nil {
		return -1, err
	}

	go func() {
		defer stream.CloseSend()
		if stdin == nil {
			return
		}
		buf := make([]byte, 32*1024)
		for {
			n, cause := stdin.Read(buf)
			if n > 0 {
				if stream.Send(&StreamRequest{Body: &StreamRequest_Stdin{Stdin: bytes.Clone(buf[:n])}}) != nil {
					return
				}
			}
			if cause != nil {
				return
			}
		}
	}()

	for {
		if resp, err = stream.Recv(); err != nil {
			return -1, err
		}
		switch body := resp.Body.(type) {
		case *StreamResponse_Stdout:
			if _, err = stdout.Write(body.Stdout); err != nil {
				return -1, err
			}
		case *StreamResponse_Stderr:
			if _, err = stderr.Write(body.Stderr); err != nil {
				return -1, err
			}
		case *StreamResponse_Exit:
			return body.Exit, nil
		}
	}
}
