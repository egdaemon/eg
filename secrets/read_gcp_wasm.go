//go:build wasip1 && wasm

package secrets

import (
	"context"
	"crypto/tls"
	"net"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func newgcpclient(ctx context.Context) (*secretmanager.Client, error) {
	gcpcreds := option.WithCredentialsJSON(envx.Base64([]byte("{}"), eg.EnvUnsafeGcloudADCB64))
	tlscreds := option.WithGRPCDialOption(
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			}),
		),
	)

	dialer := option.WithGRPCDialOption(grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return wasinet.DialContext(ctx, "tcp", addr)
	}))

	return secretmanager.NewClient(ctx, gcpcreds, dialer, tlscreds)
}
