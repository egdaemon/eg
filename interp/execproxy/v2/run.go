package execproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	v1 "github.com/egdaemon/eg/interp/execproxy"
)

// stdionames are the sockets of a process within proc/{id}, in the order the module connects to them.
var stdionames = []string{"stdin", "stdout", "stderr"}

// Run executes req through c with its stdio connected to stdin, stdout and stderr, and returns its exit code.
// it assigns req.Id and listens on proc/{id} within runtimedir, the caller's view of the module's runtime
// directory; the guest passes a listen backed by wasinet.
func Run(ctx context.Context, c v1.ProxyClient, listen func(path string) (net.Listener, error), runtimedir string, req *v1.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (code int32, err error) {
	var (
		id        = make([]byte, 4)
		proc      string
		listeners []net.Listener
		conns     []net.Conn
		l         net.Listener
		conn      net.Conn
		resp      *v1.ExecResponse
		exited    = make(chan error, 1)
		wg        sync.WaitGroup
	)

	// unix socket paths are length limited, so the id is kept short.
	rand.Read(id)
	req.Id = hex.EncodeToString(id)
	proc = filepath.Join(runtimedir, "proc", req.Id)
	if err = os.MkdirAll(proc, 0700); err != nil {
		return -1, err
	}
	defer os.RemoveAll(proc)

	for _, name := range stdionames {
		if l, err = listen(filepath.Join(proc, name)); err != nil {
			return -1, err
		}
		defer l.Close()
		listeners = append(listeners, l)
	}

	go func() {
		var cause error
		resp, cause = c.Exec(ctx, req)
		// a failed exec never connects, so closing the listeners releases the accepts below.
		for _, l := range listeners {
			l.Close()
		}
		exited <- cause
	}()

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
