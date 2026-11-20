package ttyx

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"

	xterm "golang.org/x/term"
)

// ErrNoTerminal is returned by Attach when there is no terminal to show the session on.
var ErrNoTerminal = errors.New("attaching needs stdin and stdout to be a terminal")

// Attach connects the terminal of this process to the Host listening at path: its output is shown, with what was printed
// before as a replay, and key presses are sent to it. it returns nil when the host ends, or the context is done, and leaving
// is only a disconnect: the command being hosted is not told.
func Attach(ctx context.Context, path string) error {
	return attach(ctx, path, stdin(), os.Stdout)
}

func attach(ctx context.Context, path string, in, out *os.File) error {
	if !isterminal(in) || !isterminal(out) {
		return ErrNoTerminal
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		return err
	}
	defer conn.Close()

	restore, err := Raw(in)
	if err != nil {
		return err
	}
	defer restore()

	var wmu sync.Mutex // the key presses and the size share the connection.
	send := func(kind byte, payload []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		return writeFrame(conn, kind, payload)
	}

	size := func() {
		if cols, rows, err := xterm.GetSize(int(in.Fd())); err == nil && rows > 0 && cols > 0 {
			_ = send(frameResize, encodeSize(uint16(rows), uint16(cols)))
		}
	}

	size()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	if sig, ok := sigwinch(); ok {
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, sig)
		defer signal.Stop(winch)

		go func() {
			for {
				select {
				case <-winch:
					size()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				if send(frameData, append([]byte(nil), buf[:n]...)) != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		kind, payload, err := readFrame(conn)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}

			return err
		}

		if kind == frameData {
			if _, err = out.Write(payload); err != nil {
				return err
			}
		}
	}
}
