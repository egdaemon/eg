package ffiwasinet

import (
	"context"
	"syscall"

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
		fdptr uintptr,
	) syscall.Errno {
		return wasinet.SocketOpen(wnet.Open)(ctx, m.Memory(), af, socktype, proto, fdptr)
	}).Export("socket_open").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd uint32, addr uintptr, addrlen uint32,
	) syscall.Errno {
		return wasinet.SocketBind(wnet.Bind)(ctx, m.Memory(), fd, addr, addrlen)
	}).Export("socket_bind").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uintptr,
		addrlen uint32,
	) syscall.Errno {
		return wasinet.SocketConnect(wnet.Connect)(ctx, m.Memory(), fd, addr, addrlen)
	}).Export("sock_connect").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		backlog int32,
	) syscall.Errno {
		return wasinet.SocketListen(wnet.Listen)(ctx, m.Memory(), fd, backlog)
	}).Export("sock_listen").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		level int32,
		name int32,
		valueptr uintptr,
		valuelen uint32,
	) syscall.Errno {
		return wasinet.SocketGetOpt(wnet.GetSocketOption)(ctx, m.Memory(), fd, level, name, valueptr, valuelen)
	}).Export("sock_getsockopt").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		level int32,
		name int32,
		valueptr uintptr,
		valuelen uint32,
	) syscall.Errno {
		return wasinet.SocketSetOpt(wnet.SetSocketOption)(ctx, m.Memory(), fd, level, name, valueptr, valuelen)
	}).Export("sock_setsockopt").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uintptr,
		addrlen uint32,
	) syscall.Errno {
		return wasinet.SocketLocalAddr(wnet.LocalAddr)(ctx, m.Memory(), fd, addr, addrlen)
	}).Export("sock_getlocaladdr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		fd int32,
		addr uintptr,
		addrlen uint32,
	) syscall.Errno {
		return wasinet.SocketPeerAddr(wnet.PeerAddr)(ctx, m.Memory(), fd, addr, addrlen)
	}).Export("sock_getpeeraddr").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uintptr, networklen uint32,
		addressptr uintptr, addresslen uint32,
		ipres uintptr, maxipresLen uint32,
		ipreslen uintptr,
	) syscall.Errno {
		return wasinet.SocketAddrIP(wnet.AddrIP)(ctx, m.Memory(), networkptr, networklen, addressptr, addresslen, ipres, maxipresLen, ipreslen)
	}).Export("sock_getaddrip").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		networkptr uintptr, networklen uint32,
		serviceptr uintptr, servicelen uint32,
		portptr uintptr,
	) syscall.Errno {
		return wasinet.SocketAddrPort(wnet.AddrPort)(ctx, m.Memory(), networkptr, networklen, serviceptr, servicelen, portptr)
	}).Export("sock_getaddrport").
		NewFunctionBuilder().WithFunc(func(
		ctx context.Context, m api.Module, fd, how int32,
	) syscall.Errno {
		return wasinet.SocketShutdown(wnet.Shutdown)(ctx, m.Memory(), fd, how)
	}).Export("sock_shutdown")
}
