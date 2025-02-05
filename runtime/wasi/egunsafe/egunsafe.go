package egunsafe

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/grpc"
)

func DialControlSocket(ctx context.Context) (conn *grpc.ClientConn, err error) {
	cspath := RuntimeDirectory(eg.SocketControl)
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
