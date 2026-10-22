package webrtcp2p

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/p2p/transport/webrtc/pb"
	"github.com/libp2p/go-msgio/pbio"
	"github.com/pion/datachannel"
	"github.com/pion/webrtc/v4"
)

const (
	// libp2p's webrtc message framing limits, see the multiplexing section of the webrtc spec.
	maxSendPayload    = 16000
	maxReceiveMessage = 256<<10 + 1<<10
	// flow control, writes wait for the data channel to drain below this.
	maxBuffered = 2 * maxSendPayload
	lowBuffered = maxSendPayload
	// how long a closed stream waits for the remote to acknowledge our FIN.
	maxFINACKWait = 10 * time.Second
)

var errStreamReset = errors.New("stream reset by remote")

// stream is a data channel carrying libp2p's message framing (flags + payload) as an
// io.ReadWriteCloser. it implements what a proxied protocol needs: data, FIN/FIN_ACK
// half close, RESET and STOP_SENDING. it does not implement deadlines.
type stream struct {
	dc  *webrtc.DataChannel
	raw datachannel.ReadWriteCloser

	rmu     sync.Mutex // serializes readers, guards pending, readErr
	r       pbio.Reader
	pending []byte
	readErr error

	wmu      sync.Mutex // serializes writes, guards finSent, stopped
	w        pbio.Writer
	finSent  bool
	stopped  bool // the remote asked us to stop sending, or reset the stream.
	low      chan struct{}
	acked    chan struct{} // closed once the remote acknowledged our FIN.
	ackOnce  sync.Once
	closed   chan struct{} // closed when the stream is closed locally or the data channel is gone.
	closeMux sync.Once
}

func newStream(dc *webrtc.DataChannel, raw datachannel.ReadWriteCloser) *stream {
	s := &stream{
		dc:     dc,
		raw:    raw,
		r:      pbio.NewDelimitedReader(raw, maxReceiveMessage),
		w:      pbio.NewDelimitedWriter(raw),
		low:    make(chan struct{}, 1),
		acked:  make(chan struct{}),
		closed: make(chan struct{}),
	}

	dc.SetBufferedAmountLowThreshold(lowBuffered)
	dc.OnBufferedAmountLow(func() {
		select {
		case s.low <- struct{}{}:
		default:
		}
	})

	return s
}

func (t *stream) Read(b []byte) (int, error) {
	t.rmu.Lock()
	defer t.rmu.Unlock()

	if len(b) == 0 {
		return 0, nil
	}

	for {
		if len(t.pending) > 0 {
			n := copy(b, t.pending)
			t.pending = t.pending[n:]
			return n, nil
		}

		if t.readErr != nil {
			return 0, t.readErr
		}

		var msg pb.Message
		if err := t.r.ReadMsg(&msg); err != nil {
			if err == io.EOF {
				// the channel closed without a FIN.
				err = io.ErrUnexpectedEOF
			}
			t.readErr = err
			return 0, err
		}

		t.pending = msg.Message

		// an unset flag is a plain data message, GetFlag would report the default (FIN).
		if msg.Flag == nil {
			continue
		}

		switch msg.GetFlag() {
		case pb.Message_FIN:
			t.readErr = io.EOF
			_ = t.writeFlag(pb.Message_FIN_ACK)
		case pb.Message_FIN_ACK:
			t.ackOnce.Do(func() { close(t.acked) })
		case pb.Message_STOP_SENDING:
			t.stop()
		case pb.Message_RESET:
			t.readErr = errStreamReset
			t.pending = nil
			t.stop()
		}
	}
}

func (t *stream) stop() {
	t.wmu.Lock()
	t.stopped = true
	t.wmu.Unlock()
	t.signalLow()
}

func (t *stream) signalLow() {
	select {
	case t.low <- struct{}{}:
	default:
	}
}

func (t *stream) writeFlag(flag pb.Message_Flag) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	return t.w.WriteMsg(&pb.Message{Flag: flag.Enum()})
}

func (t *stream) Write(b []byte) (n int, err error) {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	for len(b) > 0 {
		if t.finSent {
			return n, errors.New("write after close")
		}

		if t.stopped {
			return n, io.ErrClosedPipe
		}

		if t.dc.BufferedAmount() > maxBuffered {
			t.wmu.Unlock()
			select {
			case <-t.low:
			case <-t.closed:
			case <-time.After(250 * time.Millisecond):
			}
			t.wmu.Lock()

			select {
			case <-t.closed:
				return n, io.ErrClosedPipe
			default:
			}
			continue
		}

		chunk := b[:min(len(b), maxSendPayload)]
		if err = t.w.WriteMsg(&pb.Message{Message: chunk}); err != nil {
			return n, err
		}
		n += len(chunk)
		b = b[len(chunk):]
	}

	return n, nil
}

// CloseWrite signals that nothing more will be written (FIN), reads continue to work.
func (t *stream) CloseWrite() (err error) {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	if t.finSent {
		return nil
	}

	t.finSent = true
	return t.w.WriteMsg(&pb.Message{Flag: pb.Message_FIN.Enum()})
}

// Close closes the stream: it sends FIN, then waits (bounded) for the remote to acknowledge
// it before releasing the data channel so buffered data is not lost. anything the remote
// still sends is discarded, use CloseWrite to half close.
func (t *stream) Close() (err error) {
	t.wmu.Lock()
	already := t.finSent
	t.wmu.Unlock()
	if already {
		return nil
	}

	err = t.CloseWrite()
	if err != nil {
		t.release()
		return err
	}

	timer := time.AfterFunc(maxFINACKWait, func() { _ = t.release() })
	go func() {
		defer timer.Stop()
		defer t.release()

		// keep reading so the FIN_ACK is seen, the remote may still be sending.
		var discard [512]byte
		for {
			select {
			case <-t.acked:
				return
			case <-t.closed:
				return
			default:
			}

			if _, err := t.Read(discard[:]); err != nil {
				// nothing more to read, all that is left is the acknowledgement (or the timeout).
				select {
				case <-t.acked:
				case <-t.closed:
				}
				return
			}
		}
	}()

	return nil
}

// Reset abruptly terminates the stream.
func (t *stream) Reset() error {
	t.wmu.Lock()
	if !t.finSent {
		t.finSent = true
		_ = t.w.WriteMsg(&pb.Message{Flag: pb.Message_RESET.Enum()})
	}
	t.wmu.Unlock()

	return t.release()
}

func (t *stream) release() (err error) {
	t.closeMux.Do(func() {
		close(t.closed)
		err = t.raw.Close()
	})
	return err
}
