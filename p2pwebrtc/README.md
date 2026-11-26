# p2pwebrtc

libp2p private to private webrtc (`/webrtc`, [spec](https://github.com/libp2p/specs/blob/master/webrtc/webrtc.md))
for go-libp2p, which only ships webrtc-direct. a peer behind a NAT holds a relay reservation; a remote dials

    <relay>/p2p/<relay id>/p2p-circuit/webrtc/p2p/<peer id>

the two exchange SDP and ICE candidates over a `/webrtc-signaling/0.0.1` stream on the relayed connection
and continue on a direct webrtc connection. the relay only carries the signaling. it interoperates with
js-libp2p's `@libp2p/webrtc`, so a browser can dial a go peer this way.

## usage

```go
h, err := libp2p.New(
	libp2p.EnableRelay(),
	// optional: advertise <relayed addr>/webrtc next to every /p2p-circuit address.
	libp2p.AddrsFactory(p2pwebrtc.AddrsFactory(nil)),
)
t, err := p2pwebrtc.Add(h, p2pwebrtc.WithSTUN(p2pwebrtc.DefaultSTUNServers...))
```

`Add` registers the transport with the host's swarm, listens on `/webrtc` and serves the signaling protocol.
established connections are ordinary swarm connections: stream handlers, identify and `h.NewStream` use them,
and they are preferred over the limited relayed connection. remote addresses are the selected ICE
candidate, `/ip4/<ip>/udp/<port>/webrtc`.

dialing from go uses `t.Connect(ctx, peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{circuitWebRTCAddr}})`, since
the swarm hands every `/p2p-circuit` address to the relay transport. connections made by `Connect` are
recorded by the swarm as inbound, the swarm has no other entry point for a transport's connections.

## protocol notes

taken from js-libp2p's `private-to-private` implementation, which is the reference:

- signaling messages are varint length prefixed protobuf, `{ type: SDP_OFFER | SDP_ANSWER | ICE_CANDIDATE, data }`.
  candidates are `JSON.stringify(RTCIceCandidate.toJSON())`, `null` or `""` ends the candidates.
- the initiator creates an `init` datachannel so the offer has something to gather candidates for, sends the
  offer, reads the answer, then both sides trickle candidates until connected. the initiator closes `init`
  and the signaling stream.
- standard SDP, no munging. DTLS roles are the webrtc defaults (the answerer is the DTLS client).
- **no noise handshake**, unlike webrtc-direct: the peers are authenticated by the relayed connection that
  carried the SDP, and the SDP carries the DTLS fingerprints.
- streams are datachannels with an empty label, framed like webrtc-direct (`Message{flag, message}`), the
  stream code is adapted from go-libp2p (`LICENSE.go-libp2p`).

## limits

- ICE needs a path. without TURN, peers behind symmetric NATs on both ends do not connect.
- no STUN servers are configured by default, pass them with `WithSTUN` or `WithICEServers`.

## tests

    go test -race ./...     # go to go through a local relay
    ./interop/run.sh        # js-libp2p (node) to go and go to js-libp2p through a go relay, needs node >= 22
