package ffiwasinet

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
		af uint32,
		socktype uint32,
		proto uint32,
		fdptr uint32,
	) uint32 {
		return uint32(wasinet.SocketOpen(wnet.Open)(ctx, m.Memory(), af, socktype, proto, uintptr(fdptr)))
	}).Export("sock_open").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd uint32, addr uint32, addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketBind(wnet.Bind)(ctx, m.Memory(), fd, uintptr(addr), addrlen))
	}).Export("sock_bind").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketConnect(wnet.Connect)(ctx, m.Memory(), fd, uintptr(addr), addrlen))
	}).Export("sock_connect").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		backlog int32,
	) uint32 {
		return uint32(wasinet.SocketListen(wnet.Listen)(ctx, m.Memory(), fd, backlog))
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
		return uint32(wasinet.SocketGetOpt(wnet.GetSocketOption)(ctx, m.Memory(), fd, level, name, uintptr(valueptr), valuelen))
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
		return uint32(wasinet.SocketSetOpt(wnet.SetSocketOption)(ctx, m.Memory(), fd, level, name, uintptr(valueptr), valuelen))
	}).Export("sock_setsockopt").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketLocalAddr(wnet.LocalAddr)(ctx, m.Memory(), fd, uintptr(addr), addrlen))
	}).Export("sock_getlocaladdr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uint32,
		addrlen uint32,
	) uint32 {
		return uint32(wasinet.SocketPeerAddr(wnet.PeerAddr)(ctx, m.Memory(), fd, uintptr(addr), addrlen))
	}).Export("sock_getpeeraddr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uint32, networklen uint32,
		addressptr uint32, addresslen uint32,
		ipres uint32, maxipresLen uint32,
		ipreslen uint32,
	) uint32 {
		return uint32(wasinet.SocketAddrIP(wnet.AddrIP)(ctx, m.Memory(), uintptr(networkptr), networklen, uintptr(addressptr), addresslen, uintptr(ipres), maxipresLen, uintptr(ipreslen)))
	}).Export("sock_getaddrip").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uint32, networklen uint32,
		serviceptr uint32, servicelen uint32,
		portptr uint32,
	) uint32 {
		return uint32(wasinet.SocketAddrPort(wnet.AddrPort)(ctx, m.Memory(), uintptr(networkptr), networklen, uintptr(serviceptr), servicelen, uintptr(portptr)))
	}).Export("sock_getaddrport").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context, m api.Module, fd, how int32,
	) uint32 {
		return uint32(wasinet.SocketShutdown(wnet.Shutdown)(ctx, m.Memory(), fd, how))
	}).Export("sock_shutdown")
}