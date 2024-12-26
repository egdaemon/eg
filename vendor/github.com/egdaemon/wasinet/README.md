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
	"context"

	"github.com/egdaemon/wasinet"
	"github.com/egdaemon/wasinet/wnetruntime"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func Wazero(runtime wazero.Runtime) wazero.HostModuleBuilder {
	wnet := wnetruntime.Unrestricted()

	return runtime.NewHostModuleBuilder(wasinet.Namespace).
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		family int32,
	) int32 {
		return wasinet.DetermineHostAFFamily(family)
	}).Export("sock_determine_host_af_family").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		af int32,
		socktype int32,
		proto int32,
		fdptr uint32,
	) uint32 {
		errno := uint32(wasinet.SocketOpen(wnet.Open)(ctx, wnetruntime.WazeroMem(m.Memory()), af, socktype, proto, uintptr(fdptr)))
		return errno
	}).Export("sock_open").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd uint32, addr uint32, addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketBind(wnet.Bind)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_bind").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketConnect(wnet.Connect)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_connect").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		backlog int32,
	) uint32 {
		return uint32(wasinet.SocketListen(wnet.Listen)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, backlog))
	}).Export("sock_listen").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		level int32,
		name int32,
		valueptr uint32,
		valuelen uint32,
	) uint32 {
		return uint32(wasinet.SocketGetOpt(wnet.GetSocketOption)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, level, name, uintptr(valueptr), valuelen))
	}).Export("sock_getsockopt").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		level int32,
		name int32,
		valueptr uint32,
		valuelen uint32,
	) uint32 {
		return uint32(wasinet.SocketSetOpt(wnet.SetSocketOption)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, level, name, uintptr(valueptr), valuelen))
	}).Export("sock_setsockopt").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketLocalAddr(wnet.LocalAddr)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_getlocaladdr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketPeerAddr(wnet.PeerAddr)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_getpeeraddr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uint32, networklen uint32,
		addressptr uint32, addresslen uint32,
		ipres uint32, maxipresLen uint32,
		ipreslen uint32,
	) uint32 {
		return uint32(wasinet.SocketAddrIP(wnet.AddrIP)(ctx, wnetruntime.WazeroMem(m.Memory()), uintptr(networkptr), networklen, uintptr(addressptr), addresslen, uintptr(ipres), maxipresLen, uintptr(ipreslen)))
	}).Export("sock_getaddrip").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uint32, networklen uint32,
		serviceptr uint32, servicelen uint32,
		portptr uint32,
	) uint32 {
		return uint32(wasinet.SocketAddrPort(wnet.AddrPort)(ctx, wnetruntime.WazeroMem(m.Memory()), uintptr(networkptr), networklen, uintptr(serviceptr), servicelen, uintptr(portptr)))
	}).Export("sock_getaddrport").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		iovs uint32, iovslen uint32,
		addrptr uint32, addrlen uint32,
		iflags int32,
		nreadptr uint32,
		oflagsptr uint32,
	) uint32 {
		return uint32(wasinet.SocketRecvFrom(wnet.RecvFrom)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, uintptr(iovs), iovslen, uintptr(addrptr), addrlen, iflags, uintptr(nreadptr), uintptr(oflagsptr)))
	}).Export("sock_recv_from").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		iovsptr uint32, iovslen uint32,
		addrptr uint32, addrlen uint32,
		flags int32,
		nwritten uint32,
	) uint32 {
		return uint32(wasinet.SocketSendTo(wnet.SendTo)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, uintptr(iovsptr), iovslen, uintptr(addrptr), addrlen, flags, uintptr(nwritten)))
	}).Export("sock_send_to").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context, m api.Module, fd, how int32,
	) uint32 {
		return uint32(wasinet.SocketShutdown(wnet.Shutdown)(ctx, wnetruntime.WazeroMem(m.Memory()), fd, how))
	}).Export("sock_shutdown")
}
```