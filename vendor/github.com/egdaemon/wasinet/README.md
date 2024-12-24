### wasi networking functionality for golang.
This is a quick and simple socket implementation for golang in wasi that doesnt try to interopt with the wider ecosystem
that currently exists.

Due to the slow nature of committee and ecosystem politics between systems its taking too much time to have an interropt solution.

The wider ecosystem in the wild is based of wasmer edge implementation which is unnecessarily complicated to implement and runtimes like wazero
don't have the api to properly intergrate networking piecemeal in the wasi_snapshot_preview1 namespace.

Golang maintainers have little to no interest in a stopgap solution in the stdlib due to overly pedantic adherence to the go compatibility promises despite precendent for experimental features in stdlib existing in the past. 

This has resulted in fragmented wasi_snapshot_preview1 implementations where you can't extend the wasi_snapshot_preview1 namespace with networking infrastructure and inconsistent behavior between the implementations even in the same langauge ecosystem.

Now, do not misunderstand us, we understand everyones position which is why we wrote this library. its meant to be a stop gap that
is as transparent as possible until the ecosystem at large gets its act together. In the meantime this library is meant to be simple to inject,
and be runtime agnostic while enabling network functionality within a guest wasi binary written in golang and using this library.

In the meantime, I'd like to thank the authors of [wazero](https://github.com/tetratelabs/wazero), and [dispatchrun/net](https://github.com/dispatchrun/net). both wonderful pieces of software and I shameless plumbed their depths when implementing this library.

### known missing functionality.
- currently recvfrom, send_to functions are not implemented resulting in WriteMsgUnix,WriteMsgUDPAddrPort methods not working.
- unix sockets are untested.
- many socopts are untested.

these issues will be resolved if people provide written tests to the repository exposing the issues encountered or if we run into them in our own systems.

### what kinds of PRs we'll accept.
- test cases.
- implementation of missing functionality (soc options, weird non-standard *network* behaviors)
- we'd take PRs that fix the interopt with the wider ecosystem as well, its just the authors opinion that currently its not worth the
effort based on the internal api's they expose.

### usage

```golang
package main

import (
    "github.com/egdaemon/wasinet/autohijack"
)

func main() {
    http.Get("https://www.google.com")
}
```

```golang
package example

import (
    "github.com/egdaemon/wasinet/wnetruntime"
)

func Wazero(host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
    wasinet := wasinetruntime.Unrestricted()
    return host.NewFunctionBuilder().
        WithFunc(wasinet.Open).Export("socket_open").
        WithFunc(wasinet.Bind).Export("socket_bind").
        WithFunc(wasinet.Connect).Export("sock_connect").
        WithFunc(wasinet.Listen).Export("sock_listen").
        WithFunc(wasinet.GetSocketOption).Export("sock_getsockopt").
        WithFunc(wasinet.SetSocketOption).Export("sock_setsockopt").
        WithFunc(wasinet.LocalAddr).Export("sock_getlocaladdr").
        WithFunc(wasinet.PeerAddr).Export("sock_getpeeraddr").
        WithFunc(wasinet.Shutdown).Export("sock_shutdown").
        WithFunc(wasinet.AddrIP).Export("sock_getaddrip").
        WithFunc(wasinet.AddrPort).Export("sock_getaddrport")
}
```