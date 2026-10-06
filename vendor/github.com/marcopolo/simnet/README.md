## simnet

A small Go library for simulating packet networks in-process. It provides
drop-in `net.PacketConn` endpoints connected through configurable virtual links
with bandwidth, latency, and MTU constraints. Useful for testing networking code
without sockets or root privileges.

- **Drop-in API**: implements `net.PacketConn`
- **Realistic links**: per-direction bandwidth, latency, and MTU
- **Backpressure/buffering**: bandwidth–delay product aware queues
- **Routers**: perfect delivery, fixed-latency, simple firewall/NAT-like routing
- **Deterministic testing**: opt-in `synctest`-based tests for time control

### Install

```bash
go get github.com/marcopolo/simnet
```

### Quick start: high-level `Simnet`

Create a simulated network and two endpoints. Each endpoint gets a bidirectional
link with independent uplink/downlink settings. Start the network, then use the
returned `net.PacketConn`s as usual.

```go
package main

import (
    "fmt"
    "net"
    "time"

    "github.com/marcopolo/simnet"
)

func main() {
    n := &simnet.Simnet{}
    settings := simnet.NodeBiDiLinkSettings{
        Downlink: simnet.LinkSettings{BitsPerSecond: 10 * simnet.Mibps},
        Uplink:   simnet.LinkSettings{BitsPerSecond: 10 * simnet.Mibps},
        Latency:  5 * time.Millisecond,
    }

    addrA := &net.UDPAddr{IP: net.ParseIP("1.0.0.1"), Port: 9001}
    addrB := &net.UDPAddr{IP: net.ParseIP("1.0.0.2"), Port: 9002}

    client := n.NewEndpoint(addrA, settings)
    server := n.NewEndpoint(addrB, settings)

    _ = n.Start()
    defer n.Close()

    // Echo server
    go func() {
        buf := make([]byte, 1024)
        server.SetReadDeadline(time.Now().Add(2 * time.Second))
        n, src, err := server.ReadFrom(buf)
        if err != nil { return }
        server.WriteTo(append([]byte("echo: "), buf[:n]...), src)
    }()

    client.SetReadDeadline(time.Now().Add(2 * time.Second))
    _, _ = client.WriteTo([]byte("ping"), addrB)

    buf := make([]byte, 1024)
    nRead, _, _ := client.ReadFrom(buf)
    fmt.Println(string(buf[:nRead]))
}
```
### Configuration

- **`LinkSettings`**
  - `BitsPerSecond int`: bandwidth cap (bits/sec)
  - `MTU int`: maximum packet size (bytes). Oversized packets are dropped
- **`NodeBiDiLinkSettings`**
  - `Downlink LinkSettings`: settings for incoming traffic
  - `Uplink LinkSettings`: settings for outgoing traffic
  - `Latency time.Duration`: one-way latency for downlink packets
  - `LatencyFunc func(Packet) time.Duration`: optional function to compute variable latency per downlink packet
- Use `simnet.Mibps` for convenience when computing bitrates

### Routers

- **`PerfectRouter`**: instant delivery, in-memory switch
- **`FixedLatencyRouter`**: wraps perfect delivery with a fixed extra latency
- **`SimpleFirewallRouter`**: NAT/firewall-like behavior. A node must first send to a peer before inbound from that peer is allowed. You can also mark addresses as publicly reachable:

```go
fw := &simnet.SimpleFirewallRouter{}
fw.SetAddrPubliclyReachable(serverAddr)
```

### Link model (`SimulatedLink`)

Each endpoint created by `Simnet` sits behind a `SimulatedLink` that:
- Rate-limits using a token bucket (via `golang.org/x/time/rate`)
- Adds latency via a timed queue
- Drops packets over MTU
- Buffers up to the bandwidth–delay product

### Deadlines and stats

- `SimConn` implements `SetDeadline`, `SetReadDeadline`, `SetWriteDeadline`
  - Exceeded deadlines return `simnet.ErrDeadlineExceeded`
- `Stats()` returns counts of bytes/packets sent/received

### Testing

Run the standard test suite:

```bash
go test ./...
```

Some tests use Go's `synctest` experimental time control. Enable them with:

```bash
GOEXPERIMENT=synctest go test ./...
```

(requires Go 1.24+)

### License

BSD-3
