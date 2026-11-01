//go:build !wasip1

package wazeronet

import (
	"context"

	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func Module(runtime wazero.Runtime, wnet wnetruntime.Socket) wazero.HostModuleBuilder {
	return runtime.NewHostModuleBuilder(wnetruntime.Namespace).
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		family int32,
	) int32 {
		return wnetruntime.DetermineHostAFFamily(family)
	}).Export("sock_determine_host_af_family").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		af int32,
		socktype int32,
		proto int32,
		fdptr uint32,
	) uint32 {
		errno := uint32(wnetruntime.SocketOpen(wnet.Open)(ctx, Memory(m.Memory()), af, socktype, proto, uintptr(fdptr)))
		return errno
	}).Export("sock_open").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd uint32, addr uint32, addrlen uint32,
	) uint32 {
		return uint32(wnetruntime.SocketBind(wnet.Bind)(ctx, Memory(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_bind").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wnetruntime.SocketConnect(wnet.Connect)(ctx, Memory(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_connect").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		backlog int32,
	) uint32 {
		return uint32(wnetruntime.SocketListen(wnet.Listen)(ctx, Memory(m.Memory()), fd, backlog))
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
		return uint32(wnetruntime.SocketGetOpt(wnet.GetSocketOption)(ctx, Memory(m.Memory()), fd, level, name, uintptr(valueptr), valuelen))
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
		return uint32(wnetruntime.SocketSetOpt(wnet.SetSocketOption)(ctx, Memory(m.Memory()), fd, level, name, uintptr(valueptr), valuelen))
	}).Export("sock_setsockopt").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wnetruntime.SocketLocalAddr(wnet.LocalAddr)(ctx, Memory(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_getlocaladdr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wnetruntime.SocketPeerAddr(wnet.PeerAddr)(ctx, Memory(m.Memory()), fd, uintptr(addr), addrlen))
	}).Export("sock_getpeeraddr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uint32, networklen uint32,
		addressptr uint32, addresslen uint32,
		ipres uint32, maxipresLen uint32,
		ipreslen uint32,
	) uint32 {
		return uint32(wnetruntime.SocketAddrIP(wnet.AddrIP)(ctx, Memory(m.Memory()), uintptr(networkptr), networklen, uintptr(addressptr), addresslen, uintptr(ipres), maxipresLen, uintptr(ipreslen)))
	}).Export("sock_getaddrip").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uint32, networklen uint32,
		serviceptr uint32, servicelen uint32,
		portptr uint32,
	) uint32 {
		return uint32(wnetruntime.SocketAddrPort(wnet.AddrPort)(ctx, Memory(m.Memory()), uintptr(networkptr), networklen, uintptr(serviceptr), servicelen, uintptr(portptr)))
	}).Export("sock_getaddrport").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		iovs uint32, iovslen uint32,
		oobptr uint32, ooblen uint32,
		addrptr uint32, addrlen uint32,
		iflags int32,
		nreadptr uint32,
		oflagsptr uint32,
	) uint32 {
		return uint32(wnetruntime.SocketRecvFrom(wnet.RecvFrom)(ctx, Memory(m.Memory()), fd, uintptr(iovs), iovslen, uintptr(oobptr), ooblen, uintptr(addrptr), addrlen, iflags, uintptr(nreadptr), uintptr(oflagsptr)))
	}).Export("sock_recv_from").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		iovsptr uint32, iovslen uint32,
		oobptr uint32, ooblen uint32,
		addrptr uint32, addrlen uint32,
		flags int32,
		nwritten uint32,
	) uint32 {
		return uint32(wnetruntime.SocketSendTo(wnet.SendTo)(ctx, Memory(m.Memory()), fd, uintptr(iovsptr), iovslen, uintptr(oobptr), ooblen, uintptr(addrptr), addrlen, flags, uintptr(nwritten)))
	}).Export("sock_send_to").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context, m api.Module, fd, how int32,
	) uint32 {
		return uint32(wnetruntime.SocketShutdown(wnet.Shutdown)(ctx, Memory(m.Memory()), fd, how))
	}).Export("sock_shutdown").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context, m api.Module, fd int32, nfd uint32, addrptr uint32, addrlen uint32,
	) uint32 {
		return uint32(wnetruntime.SocketAccept(wnet.Accept)(ctx, Memory(m.Memory()), fd, uintptr(nfd), uintptr(addrptr), addrlen))
	}).Export("sock_accept")
}
