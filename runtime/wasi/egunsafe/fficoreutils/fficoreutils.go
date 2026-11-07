package fficoreutils

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/wasinet/wasinet"
	"github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Cmd mirrors exec.Cmd for the egcoreutils reactor. the utility's stdio is the unix sockets the reactor listens
// on in proc/{id}/fd, and its working directory is the path in proc/{id}/cwd.
type Cmd struct {
	Path   string
	Args   []string  // includes the command name as Args[0], like exec.Cmd
	Env    []string  // nil uses the module's environment, like exec.Cmd
	Dir    string    // empty uses the module's working directory
	Stdin  io.Reader // nil is an empty stdin
	Stdout io.Writer // nil discards
	Stderr io.Writer // nil discards
	ctx    context.Context
}

// Command returns the Cmd to execute the named utility with the given arguments.
func Command(ctx context.Context, name string, args ...string) *Cmd {
	return &Cmd{Path: name, Args: append([]string{name}, args...), ctx: ctx}
}

// Run executes the command and waits for it, a non zero exit is returned as an error.
func (t *Cmd) Run() (err error) {
	var (
		uid   = errorsx.Must(uuid.NewV7())
		id    = binary.BigEndian.Uint64(uid[8:])
		name  = egunsafe.PatternIsolateFrom(uid, 8, "%s")
		env   = t.Env
		cwd   string
		conns []net.Conn
		conn  net.Conn
		wg    sync.WaitGroup
	)

	proc := func(paths ...string) string {
		return egunsafe.RuntimeDirectory(append([]string{"proc", name}, paths...)...)
	}

	if env == nil {
		env = os.Environ()
	}

	if cwd, err = filepath.Abs(t.Dir); err != nil {
		return err
	}

	if err = os.MkdirAll(proc("fd"), 0700); err != nil {
		return err
	}
	// removing the directory removes the reactor's sockets with it.
	defer os.RemoveAll(proc())

	if err = errorsx.Compact(
		os.WriteFile(proc("cmdline"), []byte(strings.Join(t.Args, "\x00")), 0600),
		os.WriteFile(proc("environ"), []byte(strings.Join(env, "\x00")), 0600),
		// a file rather than a link: wasi does not permit links to absolute paths.
		os.WriteFile(proc("cwd"), []byte(cwd), 0600),
	); err != nil {
		return err
	}

	switch code := execveatinit(id); code {
	case 0:
	case ffierrors.ErrNotFound, ffierrors.ErrNotImplemented:
		return status.Errorf(codes.NotFound, "coreutils does not provide %s", t.Path)
	default:
		return ffiguest.Error(code, errorsx.Errorf("unable to start %s", t.Path))
	}

	// the reactor listens once init returns, the connections wait in its backlog until the utility accepts them.
	for _, fd := range []string{"0", "1", "2"} {
		if conn, err = wasinet.DialContext(t.ctx, "unix", proc("fd", fd)); err != nil {
			return errorsx.Wrapf(err, "unable to connect the stdio of %s", t.Path)
		}
		defer conn.Close()
		conns = append(conns, conn)
	}

	go func() {
		if t.Stdin != nil {
			io.Copy(conns[0], t.Stdin)
		}
		if cw, ok := conns[0].(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		}
	}()
	wg.Go(func() { io.Copy(langx.FirstNonZero(t.Stdout, io.Discard), conns[1]) })
	wg.Go(func() { io.Copy(langx.FirstNonZero(t.Stderr, io.Discard), conns[2]) })

	// the reactor can not block until the utility exits: the guest is a single thread, and has to copy the
	// utility's stdio meanwhile.
	for {
		switch code := execveatwait(id); code {
		case ffierrors.ErrPending:
			select {
			case <-t.ctx.Done():
				return t.ctx.Err()
			case <-time.After(time.Millisecond):
			}
		case ffierrors.ErrNotFound, ffierrors.ErrUnrecoverable:
			return ffiguest.Error(code, errorsx.Errorf("unable to wait on %s", t.Path))
		default:
			wg.Wait()
			return ffiguest.Error(code, errorsx.Errorf("%s exited with code %d", t.Path, code))
		}
	}
}
