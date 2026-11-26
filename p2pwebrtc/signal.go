package p2pwebrtc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/libp2p/go-msgio"
	"github.com/pion/webrtc/v4"
	"google.golang.org/protobuf/encoding/protowire"
)

// messageType is the type of a signaling message, see webrtc/webrtc.md in libp2p/specs:
//
//	message Message {
//	  enum Type { SDP_OFFER = 0; SDP_ANSWER = 1; ICE_CANDIDATE = 2; }
//	  optional Type type = 1;
//	  optional string data = 2;
//	}
type messageType uint64

const (
	messageSDPOffer     messageType = 0
	messageSDPAnswer    messageType = 1
	messageICECandidate messageType = 2
)

func (t messageType) String() string {
	switch t {
	case messageSDPOffer:
		return "SDP_OFFER"
	case messageSDPAnswer:
		return "SDP_ANSWER"
	case messageICECandidate:
		return "ICE_CANDIDATE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", uint64(t))
	}
}

// maxSignalingMessageSize bounds a single signaling message. an sdp is a few KiB.
const maxSignalingMessageSize = 64 << 10

type message struct {
	Type messageType
	Data string
}

func (m message) marshal() []byte {
	b := make([]byte, 0, len(m.Data)+8)
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(m.Type))
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, m.Data)
	return b
}

var errMissingType = errors.New("signaling message without a type")

func unmarshalMessage(b []byte) (m message, err error) {
	typed := false
	for len(b) > 0 {
		num, wt, n := protowire.ConsumeTag(b)
		if n < 0 {
			return m, protowire.ParseError(n)
		}
		b = b[n:]

		switch {
		case num == 1 && wt == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			m.Type, typed = messageType(v), true
			b = b[n:]
		case num == 2 && wt == protowire.BytesType:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			m.Data = v
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, wt, b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}

	if !typed {
		return m, errMissingType
	}

	return m, nil
}

// signaler reads and writes varint length prefixed signaling messages. writes are safe for
// concurrent use since ice candidates are written from pion callbacks.
type signaler struct {
	r  msgio.ReadCloser
	mu sync.Mutex
	w  msgio.WriteCloser
}

func newSignaler(rw io.ReadWriter) *signaler {
	return &signaler{
		r: msgio.NewVarintReaderSize(rw, maxSignalingMessageSize),
		w: msgio.NewVarintWriter(rw),
	}
}

func (t *signaler) write(m message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.w.WriteMsg(m.marshal())
}

func (t *signaler) read() (message, error) {
	b, err := t.r.ReadMsg()
	if err != nil {
		return message{}, err
	}
	defer t.r.ReleaseMsg(b)
	return unmarshalMessage(b)
}

// readType reads one message and requires it to be of the given type.
func (t *signaler) readType(expected messageType) (message, error) {
	m, err := t.read()
	if err != nil {
		return m, err
	}
	if m.Type != expected {
		return m, fmt.Errorf("expected %s, received %s", expected, m.Type)
	}
	return m, nil
}

// encodeCandidate encodes a candidate the way browsers do, JSON.stringify(RTCIceCandidate.toJSON()).
func encodeCandidate(c *webrtc.ICECandidate) (string, error) {
	encoded, err := json.Marshal(c.ToJSON())
	return string(encoded), err
}

// decodeCandidate decodes a candidate message. nil without an error is an end of candidates marker,
// which is either a JSON null or an empty candidate string.
func decodeCandidate(data string) (*webrtc.ICECandidateInit, error) {
	// js-libp2p treats a JSON empty string as the end of a candidate generation.
	if data == "" || data == `""` {
		return nil, nil
	}
	var c *webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		return nil, fmt.Errorf("invalid ice candidate: %w", err)
	}
	if c == nil || c.Candidate == "" {
		return nil, nil
	}
	// node-datachannel (js-libp2p in node) sends the sdp attribute line, pion expects the value.
	c.Candidate = strings.TrimPrefix(c.Candidate, "a=")
	return c, nil
}
