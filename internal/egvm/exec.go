package egvm

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"google.golang.org/protobuf/encoding/protodelim"
)

// Port is the vsock port vminit serves commands on.
const Port = 1024

// NFSPort is the vsock port the host serves the machine's root on. vminit forwards the guest's NFS client to it from
// the same port on the guest's loopback.
const NFSPort = 2049

// The streams of a command, each its own connection to the agent. A connection starts with a header naming the command
// and the stream it carries. The control stream carries the command's execproxy.ExecRequest from the host and its
// execproxy.ExecResponse back, both length delimited; the host closing it before the response kills the command.
const (
	StreamStdin uint8 = iota
	StreamStdout
	StreamStderr
	StreamControl
)

// Header begins every connection to the agent.
type Header struct {
	ID     uint64
	Stream uint8
}

// WriteHeader writes the header of the stream of command id to w.
func WriteHeader(w io.Writer, id uint64, stream uint8) error {
	return binary.Write(w, binary.BigEndian, Header{ID: id, Stream: stream})
}

// ReadHeader reads the header a connection starts with.
func ReadHeader(r io.Reader) (h Header, err error) {
	return h, binary.Read(r, binary.BigEndian, &h)
}

// connector opens the stream of command id to the agent.
type connector func(ctx context.Context, id uint64, stream uint8) (net.Conn, error)

// command runs req through the agent connect reaches and waits for it, with no input and its output
// discarded unless options say otherwise. A command that exits with other than 0 returns an
// ExitError. Cancelling ctx kills the command.
func command(ctx context.Context, connect connector, req *execproxy.ExecRequest, options ...ExecOption) (err error) {
	var (
		stdio   = newstdio(options...)
		id      uint64
		control net.Conn
		streams [3]net.Conn
		resp    execproxy.ExecResponse
		wg      sync.WaitGroup
	)

	if err = binary.Read(rand.Reader, binary.BigEndian, &id); err != nil {
		return err
	}

	if control, err = connect(ctx, id, StreamControl); err != nil {
		return err
	}
	defer control.Close()
	stop := context.AfterFunc(ctx, func() { control.Close() })
	defer stop()

	if _, err = protodelim.MarshalTo(control, req); err != nil {
		return errorsx.Wrap(err, "unable to send the command")
	}

	for i := range streams {
		if streams[i], err = connect(ctx, id, uint8(i)); err != nil {
			return err
		}
		defer streams[i].Close()
	}

	// the stdin stream is a connection of its own, so closing it is the command's end of input.
	go func() {
		if stdio.stdin != nil {
			io.Copy(streams[StreamStdin], stdio.stdin)
		}
		streams[StreamStdin].Close()
	}()
	wg.Go(func() { io.Copy(stdio.stdout, streams[StreamStdout]) })
	wg.Go(func() { io.Copy(stdio.stderr, streams[StreamStderr]) })

	if err = protodelim.UnmarshalFrom(bufio.NewReader(control), &resp); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errorsx.Wrap(err, "unable to read the command's exit")
	}
	wg.Wait()

	if resp.Exit != 0 {
		return ExitError{Code: resp.Exit}
	}
	return nil
}
