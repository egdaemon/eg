package ffiexec

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/grpc"
)

func Command(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	cc, err := egunsafe.DialModuleControlSocket(ctx)
	if err != nil {
		return err
	}
	svc := execproxy.NewProxyClient(cc)

	_, err = svc.Exec(ctx, &execproxy.ExecRequest{
		Cmd:         cmd,
		Dir:         dir,
		Arguments:   args,
		Environment: environ,
	})

	return err
}

// Stream runs cmd on the host with its stdio connected to stdin, stdout and stderr, and returns its exit code.
// the guest listens on proc/{id}/{stdin,stdout,stderr} in the runtime directory and the host connects the
// command to them, so its output arrives as it is written.
func Stream(ctx context.Context, dir string, environ []string, cmd string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (code int32, err error) {
	var (
		id        = make([]byte, 4)
		proc      string
		listeners []net.Listener
		conns     []net.Conn
		l         net.Listener
		conn      net.Conn
		cc        *grpc.ClientConn
		resp      *execproxy.ExecResponse
		exited    = make(chan error, 1)
		wg        sync.WaitGroup
	)

	// unix socket paths are length limited, so the id is kept short.
	rand.Read(id)
	proc = egunsafe.RuntimeDirectory("proc", hex.EncodeToString(id))
	if err = os.MkdirAll(proc, 0700); err != nil {
		return -1, err
	}
	defer os.RemoveAll(proc)

	for _, name := range []string{"stdin", "stdout", "stderr"} {
		if l, err = wasinet.Listen(ctx, "unix", filepath.Join(proc, name)); err != nil {
			return -1, err
		}
		defer l.Close()
		listeners = append(listeners, l)
	}

	if cc, err = egunsafe.DialModuleControlSocket(ctx); err != nil {
		return -1, err
	}
	defer cc.Close()

	go func() {
		var cause error
		resp, cause = execproxy.NewProxyClient(cc).Exec(ctx, &execproxy.ExecRequest{
			Id:          hex.EncodeToString(id),
			Cmd:         cmd,
			Dir:         dir,
			Arguments:   args,
			Environment: environ,
		})
		// a failed exec never connects, so closing the listeners releases the accepts below.
		for _, l := range listeners {
			l.Close()
		}
		exited <- cause
	}()

	// the host connects stdin, stdout and stderr in that order.
	for _, l = range listeners {
		if conn, err = l.Accept(); err != nil {
			break
		}
		defer conn.Close()
		conns = append(conns, conn)
	}

	if len(conns) == len(listeners) {
		go func() {
			if stdin != nil {
				io.Copy(conns[0], stdin)
			}
			conns[0].(interface{ CloseWrite() error }).CloseWrite()
		}()
		wg.Go(func() { io.Copy(stdout, conns[1]) })
		wg.Go(func() { io.Copy(stderr, conns[2]) })
	}

	err = <-exited
	wg.Wait()

	if err != nil {
		return -1, err
	}

	return resp.Exit, nil
}
