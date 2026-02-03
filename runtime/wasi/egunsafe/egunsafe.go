// Package provides some internal only functionality
// not under compatability promises.
package egunsafe

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"path/filepath"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/grpc"
)

// dial the control socket for executing various functionality that is too slow or impedes concurrency.
func DialControlSocket(ctx context.Context) (conn *grpc.ClientConn, err error) {
	// log.Println("DIALING CONTROL SOCKET INITIATED")
	// defer log.Println("DIALING CONTROL SOCKET COMPLETED")

	cspath := RuntimeDirectory(eg.SocketControl)
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure(), grpc.WithDialer(func(s string, d time.Duration) (net.Conn, error) {
		dctx, done := context.WithTimeout(ctx, d)
		defer done()
		proto, address, _ := strings.Cut(s, "://")
		return wasinet.DialContext(dctx, proto, address)
	}))
}

// dial the control socket for modules.
func DialModuleControlSocket(ctx context.Context) (conn *grpc.ClientConn, err error) {
	cspath := envx.String(RuntimeDirectory(eg.SocketControl), eg.EnvComputeModuleSocket)
	// log.Println("DIALING MODULE SOCKET INITIATED")
	// defer log.Println("DIALING MODULE SOCKET COMPLETED")
	// envx.Debug(os.Environ()...)
	// log.Println("DERP DERP", cspath)
	// log.Println("default", RuntimeDirectory(eg.SocketControl))
	// fsx.PrintDir(os.DirFS(RuntimeDirectory()))

	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure(), grpc.WithDialer(func(s string, d time.Duration) (net.Conn, error) {
		dctx, done := context.WithTimeout(ctx, d)
		defer done()
		proto, address, _ := strings.Cut(s, "://")
		return wasinet.DialContext(dctx, proto, address)
	}))
}

func RuntimeDirectory(paths ...string) string {
	return eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Join(paths...))
}

func UnroutablePrefix() netip.Prefix {
	return netip.PrefixFrom(netip.IPv6Unspecified(), 128)
}

// resolve the netip.Prefixes of the host. returns a slice containing only
// the unroutable prefix on error.
func HostPrefixes() []netip.Prefix {
	ips, err := net.LookupIP("host.containers.internal")
	if err != nil || len(ips) == 0 {
		log.Println("failed to lookup host ip - return void prefix", err)
		return []netip.Prefix{UnroutablePrefix()}
	}

	prefixes := make([]netip.Prefix, 0, len(ips))
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}

		// Map to 32 for IPv4, 128 for IPv6
		bits := 128
		if addr.Is4() {
			bits = 32
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, bits))
	}

	if len(prefixes) == 0 {
		return []netip.Prefix{UnroutablePrefix()}
	}

	return prefixes
}
