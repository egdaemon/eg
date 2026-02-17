//go:build wasip1 && wasm

package secrets

import (
	"context"
	"net"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

func newgcpclient(ctx context.Context) (*secretmanager.Client, error) {
	dialer := option.WithGRPCDialOption(grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return wasinet.DialContext(ctx, "tcp", addr)
	}))

	return secretmanager.NewClient(ctx, dialer)
}
