package p2pwebrtc

import (
	"bytes"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestMessageRoundTrip(t *testing.T) {
	for _, m := range []message{
		{Type: messageSDPOffer, Data: "v=0\r\n"},
		{Type: messageSDPAnswer, Data: ""},
		{Type: messageICECandidate, Data: `{"candidate":"candidate:1 1 UDP 1 127.0.0.1 1 typ host"}`},
	} {
		decoded, err := unmarshalMessage(m.marshal())
		require.NoError(t, err)
		require.Equal(t, m, decoded)
	}
}

func TestMessageWireFormat(t *testing.T) {
	// js-libp2p (protons) encodes an explicitly set enum zero value.
	require.Equal(t, []byte{0x08, 0x00, 0x12, 0x02, 'h', 'i'}, message{Type: messageSDPOffer, Data: "hi"}.marshal())

	// unknown fields are skipped, the data field is optional.
	m, err := unmarshalMessage([]byte{0x18, 0x05, 0x08, 0x02})
	require.NoError(t, err)
	require.Equal(t, message{Type: messageICECandidate}, m)

	_, err = unmarshalMessage([]byte{0x12, 0x02, 'h', 'i'})
	require.ErrorIs(t, err, errMissingType)

	_, err = unmarshalMessage([]byte{0x08})
	require.Error(t, err)
}

func TestSignalerFraming(t *testing.T) {
	var buf bytes.Buffer
	s := newSignaler(&buf)
	require.NoError(t, s.write(message{Type: messageSDPOffer, Data: "offer"}))
	require.NoError(t, s.write(message{Type: messageICECandidate, Data: "null"}))
	// varint length prefix followed by the message.
	require.Equal(t, byte(len(message{Type: messageSDPOffer, Data: "offer"}.marshal())), buf.Bytes()[0])

	m, err := s.readType(messageSDPOffer)
	require.NoError(t, err)
	require.Equal(t, "offer", m.Data)

	_, err = s.readType(messageSDPAnswer)
	require.ErrorContains(t, err, "expected SDP_ANSWER, received ICE_CANDIDATE")
}

func TestDecodeCandidate(t *testing.T) {
	for _, eoc := range []string{"", "null", `""`, `{"candidate":""}`} {
		c, err := decodeCandidate(eoc)
		require.NoError(t, err, eoc)
		require.Nil(t, c, eoc)
	}

	c, err := decodeCandidate(`{"candidate":"candidate:1 1 UDP 2114977791 192.168.1.2 56201 typ host","sdpMid":"0","sdpMLineIndex":0,"usernameFragment":"abcd"}`)
	require.NoError(t, err)
	require.Equal(t, "candidate:1 1 UDP 2114977791 192.168.1.2 56201 typ host", c.Candidate)
	require.Equal(t, "0", *c.SDPMid)
	require.Equal(t, uint16(0), *c.SDPMLineIndex)

	c, err = decodeCandidate(`{"candidate":"a=candidate:1 1 UDP 1 192.168.1.2 1 typ host","sdpMid":"0"}`)
	require.NoError(t, err)
	require.Equal(t, "candidate:1 1 UDP 1 192.168.1.2 1 typ host", c.Candidate)

	_, err = decodeCandidate("{")
	require.Error(t, err)
}

func TestAddrs(t *testing.T) {
	relayed := ma.StringCast("/ip4/1.2.3.4/tcp/1/p2p/12D3KooWEoHskKp6Z7QUor2PNc9d1GtonhHGpyPq3L5UMGd6sFGC/p2p-circuit")
	direct := ma.StringCast("/ip4/1.2.3.4/tcp/2")

	got := Addrs([]ma.Multiaddr{direct, ListenAddr, relayed})
	require.ElementsMatch(t, []ma.Multiaddr{direct, relayed, relayed.Encapsulate(ListenAddr)}, got)

	wrapped := AddrsFactory(func(in []ma.Multiaddr) []ma.Multiaddr { return in[:1] })
	require.Equal(t, []ma.Multiaddr{direct}, wrapped([]ma.Multiaddr{direct, relayed}))
}

func TestSignalingAddrs(t *testing.T) {
	target := ma.StringCast("/ip4/1.2.3.4/tcp/1/p2p/12D3KooWEoHskKp6Z7QUor2PNc9d1GtonhHGpyPq3L5UMGd6sFGC/p2p-circuit/webrtc/p2p/12D3KooWBJkiYxmnKZkNGRma1BfC62zcxchBUmnggzi2kcgJvPvP")
	require.Equal(t, []ma.Multiaddr{
		ma.StringCast("/ip4/1.2.3.4/tcp/1/p2p/12D3KooWEoHskKp6Z7QUor2PNc9d1GtonhHGpyPq3L5UMGd6sFGC/p2p-circuit"),
	}, SignalingAddrs(target, ListenAddr))
}

func TestCandidateAddr(t *testing.T) {
	require.Equal(t, ListenAddr, candidateaddr(nil))
}
