package ttyx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
)

// Mux shares the user's terminal between commands running at the same time, the way tmux or screen do. every command
// is given a pseudo terminal of its own (a Session) so its output is processed by its own line discipline and it can
// change its own mode, e.g.) raw, without affecting the others. the mux copies the output of every session to the
// user's terminal one whole line at a time, and the key presses typed on the user's terminal to a single session, the
// foreground one.
//
// without a terminal (cron, ci, piped) the mux does nothing and sessions are the process's own stdin, stdout and
// stderr. a nil *Mux behaves the same.
type Mux struct {
	ctx     context.Context
	done    context.CancelFunc
	ctl     *os.File // the user's terminal, for its mode and window size.
	in      io.ReadCloser
	out     io.Writer
	restore func()
	wmu     sync.Mutex // serializes writes to out.
	smu     sync.Mutex
	stack   []*Session // open sessions, the last is the foreground.
	wg      sync.WaitGroup
}

// NewMux takes the terminal of the process when both stdin and stdout are one and pseudo terminals are supported.
// Close must be called to give the terminal back.
func NewMux(ctx context.Context) *Mux {
	if !isterminal(stdin()) || !isterminal(os.Stdout) {
		return nil
	}

	path, _ := devicepath()
	ctl, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		debugx.Println("terminal mux disabled, unable to open the terminal", path, err)
		return nil
	}

	// a descriptor of its own, opened rather than duplicated, is pollable without changing stdin, so Close stops the read.
	in, err := os.Open(path)
	if err != nil {
		ctl.Close()
		debugx.Println("terminal mux disabled, unable to open the terminal", path, err)
		return nil
	}

	m, err := newmux(ctx, ctl, in, os.Stdout)
	if err != nil {
		in.Close()
		ctl.Close()
		debugx.Println("terminal mux disabled", err)
		return nil
	}

	return m
}

func newmux(ctx context.Context, ctl *os.File, in io.ReadCloser, out io.Writer) (_ *Mux, err error) {
	// pseudo terminals must be available before the terminal is taken.
	parent, child, err := openpty()
	if err != nil {
		return nil, err
	}
	parent.Close()
	child.Close()

	m := &Mux{ctl: ctl, in: in, out: out}
	m.ctx, m.done = context.WithCancel(ctx)

	// key presses are delivered as typed, the session's own line discipline echoes and edits them. ctrl-c still
	// interrupts eg as a whole.
	if m.restore, err = cbreak(ctl); err != nil {
		return nil, err
	}

	m.wg.Add(2)
	go m.input()
	go m.winch()

	return m, nil
}

// Close gives the terminal back in the mode it was found, closing any session still open.
func (t *Mux) Close() error {
	if t == nil {
		return nil
	}

	t.smu.Lock()
	open := append([]*Session(nil), t.stack...)
	t.smu.Unlock()
	for _, s := range open {
		s.Close()
	}

	t.done()
	err := t.in.Close()
	t.wg.Wait()
	t.restore()
	return errors.Join(err, t.ctl.Close())
}

// Stdout is a writer to the user's terminal that never splits a line written by a session.
func (t *Mux) Stdout() io.Writer {
	if t == nil {
		return os.Stdout
	}

	return writerfn(t.write)
}

// Stderr is Stdout, a session's stderr and stdout are the same terminal.
func (t *Mux) Stderr() io.Writer {
	if t == nil {
		return os.Stderr
	}

	return writerfn(t.write)
}

// Session allocates a pseudo terminal for a command and makes it the foreground session.
// when no terminal is available, or the pseudo terminal cannot be created, it is the process's own files.
func (t *Mux) Session() *Session {
	if t == nil {
		return &Session{}
	}

	parent, child, err := openpty()
	if err != nil {
		log.Println("unable to allocate a terminal for the command, using the shared one", err)
		return &Session{}
	}

	s := &Session{m: t, parent: parent, child: child, pumped: make(chan struct{})}
	if err := resize(t.ctl, parent); err != nil {
		debugx.Println("unable to size the session terminal", err)
	}

	t.smu.Lock()
	t.stack = append(t.stack, s)
	t.smu.Unlock()

	go s.pump()

	return s
}

// Foreground gives the key presses typed on the terminal to s.
func (t *Mux) Foreground(s *Session) {
	if t == nil || s == nil || s.m != t {
		return
	}

	t.smu.Lock()
	defer t.smu.Unlock()
	if i := t.index(s); i >= 0 {
		t.stack = append(append(t.stack[:i:i], t.stack[i+1:]...), s)
	}
}

func (t *Mux) foreground() *Session {
	t.smu.Lock()
	defer t.smu.Unlock()
	if len(t.stack) == 0 {
		return nil
	}
	return t.stack[len(t.stack)-1]
}

// index of s in the stack, the caller holds smu.
func (t *Mux) index(s *Session) int {
	for i, c := range t.stack {
		if c == s {
			return i
		}
	}
	return -1
}

func (t *Mux) remove(s *Session) {
	t.smu.Lock()
	defer t.smu.Unlock()
	if i := t.index(s); i >= 0 {
		t.stack = append(t.stack[:i:i], t.stack[i+1:]...)
	}
}

func (t *Mux) write(p []byte) (int, error) {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	return t.out.Write(p)
}

// input copies key presses to the foreground session, they are dropped when there is none.
func (t *Mux) input() {
	defer t.wg.Done()
	buf := make([]byte, 1024)
	for {
		n, err := t.in.Read(buf)
		if n > 0 {
			if s := t.foreground(); s != nil {
				if _, werr := s.parent.Write(buf[:n]); werr != nil {
					debugx.Println("unable to forward input to the session", werr)
				}
			}
		}

		if err != nil {
			return
		}
	}
}

// winch gives every session the size of the terminal whenever it changes.
func (t *Mux) winch() {
	defer t.wg.Done()
	sig, ok := sigwinch()
	if !ok {
		return
	}

	changed := make(chan os.Signal, 1)
	signal.Notify(changed, sig)
	defer signal.Stop(changed)

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-changed:
			t.smu.Lock()
			open := append([]*Session(nil), t.stack...)
			t.smu.Unlock()
			for _, s := range open {
				if err := resize(t.ctl, s.parent); err != nil {
					debugx.Println("unable to resize the session terminal", err)
				}
			}
		}
	}
}

// Session is the terminal of one command. hand Stdin, Stdout and Stderr to the command and Close it once the command
// has finished.
type Session struct {
	m      *Mux
	parent *os.File
	child  *os.File
	pumped chan struct{}
	once   sync.Once
}

// Stdin of the command, it receives key presses while the session is in the foreground.
func (t *Session) Stdin() *os.File {
	if t.child == nil {
		return os.Stdin
	}
	return t.child
}

func (t *Session) Stdout() *os.File {
	if t.child == nil {
		return os.Stdout
	}
	return t.child
}

func (t *Session) Stderr() *os.File {
	if t.child == nil {
		return os.Stderr
	}
	return t.child
}

// Close releases the terminal of the session once its remaining output is written.
func (t *Session) Close() error {
	if t.child == nil {
		return nil
	}

	var err error
	t.once.Do(func() {
		t.m.remove(t)
		err = t.child.Close()

		// the parent reads until every descriptor of the child is closed, a process left running in the background
		// with one would hold it forever.
		select {
		case <-t.pumped:
		case <-time.After(time.Second):
		}

		err = errors.Join(err, t.parent.Close())
		<-t.pumped
	})

	return err
}

// pump writes the output of the session to the terminal. whole lines are written at once, so the output of sessions
// running at the same time never interleaves within a line. a partial line, e.g.) a prompt or a progress bar, is
// written once the session stops writing for a moment: shortly for the foreground session, which the user is
// typing into, longer for the others.
func (t *Session) pump() {
	defer close(t.pumped)

	const (
		limit      = 4096
		foreground = 25 * time.Millisecond
		background = 500 * time.Millisecond
	)

	var (
		buf     = make([]byte, limit)
		pending []byte
	)

	flush := func() {
		if len(pending) > 0 {
			_, _ = t.m.write(pending)
			pending = pending[:0]
		}
	}

	for {
		n, err := t.parent.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			if i := bytes.LastIndexByte(pending, '\n'); i >= 0 {
				_, _ = t.m.write(pending[:i+1])
				pending = append(pending[:0], pending[i+1:]...)
			}

			if len(pending) >= limit {
				flush()
			}
		}

		if errors.Is(err, os.ErrDeadlineExceeded) {
			flush()
			err = nil
		}

		if err != nil {
			flush()
			return
		}

		idle := time.Time{}
		if len(pending) > 0 && t.m.foreground() == t {
			idle = time.Now().Add(foreground)
		} else if len(pending) > 0 {
			idle = time.Now().Add(background)
		}
		_ = t.parent.SetReadDeadline(idle)
	}
}

type writerfn func([]byte) (int, error)

func (t writerfn) Write(p []byte) (int, error) {
	return t(p)
}
