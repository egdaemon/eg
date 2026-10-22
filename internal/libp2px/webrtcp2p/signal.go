// Package webrtcp2p implements the answering side of the libp2p browser-to-private-node
// webrtc transport (https://github.com/libp2p/specs/blob/master/webrtc/webrtc.md).
//
// go-libp2p only implements webrtc-direct, which needs a publicly reachable udp port, so
// runners behind a NAT cannot be dialed by a browser. this package lets a runner answer the
// browser's /webrtc-signaling/0.0.1 request that arrives over a relayed connection, establish
// the peer connection, and serve protocol streams carried on its data channels.
//
// it is intentionally not a swarm transport: the resulting connections are not visible to the
// libp2p host (no resource manager accounting, no identify) and only the protocols registered
// with New are served.
package webrtcp2p

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"

	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/encoding/protowire"
)

// SignalProtocol is the protocol browsers open on the relayed connection to start the exchange.
const SignalProtocol protocol.ID = "/webrtc-signaling/0.0.1"

// maxSignalSize bounds a single signaling message, an sdp with many candidates is a few kB.
const maxSignalSize = 64 << 10

type signalType int32

const (
	signalSDPOffer     signalType = 0
	signalSDPAnswer    signalType = 1
	signalICECandidate signalType = 2
)

// signal is the signaling Message from the spec:
//
//	message Message { optional Type type = 1; optional string data = 2; }
//
// encoded by hand to avoid a generated proto for two fields.
type signal struct {
	kind signalType
	data string
}

func readSignal(r *bufio.Reader) (m signal, err error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return m, err
	}

	if n > maxSignalSize {
		return m, errors.New("signaling message too large")
	}

	buf := make([]byte, n)
	if _, err = io.ReadFull(r, buf); err != nil {
		return m, err
	}

	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return m, protowire.ParseError(n)
		}
		buf = buf[n:]

		switch {
		case num == 1 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(buf)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			m.kind = signalType(v)
			buf = buf[n:]
		case num == 2 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			m.data = string(v)
			buf = buf[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, buf)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			buf = buf[n:]
		}
	}

	return m, nil
}

func writeSignal(w io.Writer, m signal) error {
	var body []byte
	body = protowire.AppendTag(body, 1, protowire.VarintType)
	body = protowire.AppendVarint(body, uint64(m.kind))
	body = protowire.AppendTag(body, 2, protowire.BytesType)
	body = protowire.AppendString(body, m.data)

	// a single write, messages from the candidate callback and the answer share the stream.
	return writeAll(w, protowire.AppendVarint(nil, uint64(len(body))), body)
}

func writeAll(w io.Writer, parts ...[]byte) error {
	var buf []byte
	for _, p := range parts {
		buf = append(buf, p...)
	}
	_, err := w.Write(buf)
	return err
}
