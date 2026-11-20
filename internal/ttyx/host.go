package ttyx

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
)

// replaysize is how much of the most recent output a client that attaches is shown.
const replaysize = 1 << 20

// Host owns a pseudo terminal that outlives whoever is looking at it, the way dtach does. the command is given the
// child side as its terminal (Child) and clients attach over a unix socket to see its output and type into it, see Attach.
// any number of clients can be attached, and one leaving is only a disconnect: nothing is written to the pseudo terminal,
// so the command sees no change, and with no client the output is still kept for the next one and copied to the mirror.
type Host struct {
	path   string
	ln     net.Listener
	parent *os.File
	child  *os.File
	mirror io.Writer

	mu      sync.Mutex // guards clients and replay, and orders the output so a client sees the replay and then everything after it.
	clients map[*hostclient]struct{}
	replay  []byte

	wmu    sync.Mutex // serializes writes to the pseudo terminal.
	wg     sync.WaitGroup
	once   sync.Once
	closed chan struct{}
}

type hostclient struct {
	conn  net.Conn
	queue chan []byte
	quit  chan struct{} // closed once the client is gone, so what writes to it stops.
}

// NewHost allocates a pseudo terminal and listens for clients on the unix socket at path, the directory is created
// private to the user. mirror receives a copy of everything the command prints, e.g.) the log, it may be nil.
// the host ends with ctx, or when closed.
func NewHost(ctx context.Context, path string, mirror io.Writer) (*Host, error) {
	parent, child, err := openpty()
	if err != nil {
		return nil, err
	}

	closeptys := func() {
		parent.Close()
		child.Close()
	}

	// a size for the command to start with, the first client that attaches gives it its own.
	_ = setsize(parent, 24, 80)

	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		closeptys()
		return nil, err
	}

	// a socket left by a host that was killed.
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		closeptys()
		return nil, err
	}

	if err = os.Chmod(path, 0600); err != nil {
		ln.Close()
		closeptys()
		return nil, err
	}

	h := &Host{
		path:    path,
		ln:      ln,
		parent:  parent,
		child:   child,
		mirror:  mirror,
		clients: map[*hostclient]struct{}{},
		closed:  make(chan struct{}),
	}

	h.wg.Add(2)
	go h.pump()
	go h.accept()

	go func() {
		select {
		case <-ctx.Done():
			h.Close()
		case <-h.closed:
		}
	}()

	return h, nil
}

// Child is the terminal handed to the command as its stdin, stdout and stderr. nil for a nil host.
func (t *Host) Child() *os.File {
	if t == nil {
		return nil
	}

	return t.child
}

// Path is the socket clients attach to, blank for a nil host: a run that has no terminal hosted.
func (t *Host) Path() string {
	if t == nil {
		return ""
	}

	return t.path
}

// Close ends the host: clients are disconnected, the pseudo terminal is closed and the socket is removed. a nil host has nothing to close.
func (t *Host) Close() error {
	if t == nil {
		return nil
	}

	t.once.Do(func() {
		close(t.closed)
		t.ln.Close()

		t.mu.Lock()
		for c := range t.clients {
			c.conn.Close()
		}
		t.mu.Unlock()

		t.parent.Close()
		t.child.Close()
		t.wg.Wait()
		_ = os.Remove(t.path)
	})

	return nil
}

// pump copies what the command prints to the clients, the replay and the mirror.
func (t *Host) pump() {
	defer t.wg.Done()

	buf := make([]byte, 32<<10)
	for {
		n, err := t.parent.Read(buf)
		if n > 0 {
			t.output(append([]byte(nil), buf[:n]...))
		}

		if err != nil {
			debugx.Println("terminal host output ended", err)
			return
		}
	}
}

func (t *Host) output(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.mirror != nil {
		_, _ = t.mirror.Write(data)
	}

	t.replay = trimreplay(append(t.replay, data...))

	for c := range t.clients {
		select {
		case c.queue <- data:
		default:
			// a client that does not keep up is dropped, the command never waits on one.
			delete(t.clients, c)
			c.conn.Close()
		}
	}
}

// trimreplay keeps the last replaysize bytes, starting after a line break so no half line, or half of an escape sequence, is replayed.
func trimreplay(b []byte) []byte {
	if len(b) <= replaysize {
		return b
	}

	b = b[len(b)-replaysize:]
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[i+1:]
	}

	return append([]byte(nil), b...)
}

func (t *Host) accept() {
	defer t.wg.Done()

	for {
		conn, err := t.ln.Accept()
		if err != nil {
			return
		}

		t.wg.Add(1)
		go t.serve(conn)
	}
}

func (t *Host) serve(conn net.Conn) {
	defer t.wg.Done()
	defer conn.Close()

	c := &hostclient{conn: conn, queue: make(chan []byte, 1024), quit: make(chan struct{})}

	t.mu.Lock()
	// a client accepted as the host closes is not in the set Close disconnects, it is turned away here instead.
	select {
	case <-t.closed:
		t.mu.Unlock()
		return
	default:
	}

	// a terminal reset so the new tab starts clean, then what was printed, then everything after it.
	c.queue <- []byte("\x1bc")
	if len(t.replay) > 0 {
		c.queue <- append([]byte(nil), t.replay...)
	}
	t.clients[c] = struct{}{}
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.clients, c)
		t.mu.Unlock()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case data := <-c.queue:
				if err := writeFrame(conn, frameData, data); err != nil {
					conn.Close()
					return
				}
			case <-c.quit:
				return
			case <-t.closed:
				return
			}
		}
	}()

	for {
		kind, payload, err := readFrame(conn)
		if err != nil {
			// the client left, or was dropped. that is all, the pseudo terminal is not touched.
			conn.Close()
			close(c.quit)
			<-done
			return
		}

		switch kind {
		case frameData:
			t.wmu.Lock()
			_, err = t.parent.Write(payload)
			t.wmu.Unlock()
			if err != nil {
				debugx.Println("terminal host input failed", err)
			}
		case frameResize:
			if rows, cols, ok := decodeSize(payload); ok {
				t.resize(rows, cols)
			}
		}
	}
}

// resize gives the pseudo terminal the size of the client that asked. the command's terminal is told of it by signalling the
// process the way a terminal does, the handler that follows the size of a terminal for the container reads it from the child.
// a program only repaints when the size changed, so a width that is unchanged is changed for a moment and put back, which is
// what makes a client that attaches to a full-screen program see it drawn.
func (t *Host) resize(rows, cols uint16) {
	if rows == 0 || cols == 0 {
		return
	}

	if cols > 1 {
		if err := setsize(t.parent, rows, cols-1); err == nil {
			winch()
			time.Sleep(50 * time.Millisecond)
		}
	}

	if err := setsize(t.parent, rows, cols); err != nil {
		log.Println("unable to resize the terminal", err)
		return
	}

	winch()
}

// winch signals this process that the size of its terminal changed.
func winch() {
	sig, ok := sigwinch()
	if !ok {
		return
	}

	if p, err := os.FindProcess(os.Getpid()); err == nil {
		_ = p.Signal(sig)
	}
}

// SocketPath is where the host of the terminal of a session listens: the runtime directory of the user when there is one,
// else their cache directory, else the temporary directory. kept short, unix socket paths are limited to about 100 bytes.
func SocketPath(id string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			dir = cache
		} else {
			dir = os.TempDir()
		}
	}

	return filepath.Join(dir, "eg", "pty.d", id+".sock")
}
