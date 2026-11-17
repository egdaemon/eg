package environ_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/interp/environ"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestService(t *testing.T) {
	secret := jwtx.JWTSecretSource(func() []byte { return []byte("0123456789abcdef0123456789abcdef") })
	other := jwtx.JWTSecretSource(func() []byte { return []byte("fedcba9876543210fedcba9876543210") })

	dir := t.TempDir()
	envpath := filepath.Join(dir, "environ.env")
	require.NoError(t, os.WriteFile(envpath, []byte("FOO=bar\nBAZ=qux\n"), 0600))

	svc := environ.NewService(envpath, secret)

	lis, err := net.Listen("unix", filepath.Join(dir, "c.socket"))
	require.NoError(t, err)
	srv := grpc.NewServer()
	svc.Bind(srv)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("unix://"+lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	client := environ.NewEnvironClient(conn)

	good, err := environ.NewToken(secret, "run", time.Minute)
	require.NoError(t, err)
	wrongsecret, err := environ.NewToken(other, "run", time.Minute)
	require.NoError(t, err)
	expired, err := environ.NewToken(secret, "run", -time.Minute)
	require.NoError(t, err)

	t.Run("returns raw bytes", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "bearer "+good)
		resp, err := client.Get(ctx, &environ.GetRequest{})
		require.NoError(t, err)
		require.Equal(t, "FOO=bar\nBAZ=qux\n", string(resp.Environ))
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := client.Get(context.Background(), &environ.GetRequest{})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("wrong secret", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "bearer "+wrongsecret)
		_, err := client.Get(ctx, &environ.GetRequest{})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("expired", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "bearer "+expired)
		_, err := client.Get(ctx, &environ.GetRequest{})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("missing file", func(t *testing.T) {
		missing := environ.NewService(filepath.Join(dir, "nope.env"), secret)
		msrv := grpc.NewServer()
		missing.Bind(msrv)
		mlis, err := net.Listen("unix", filepath.Join(dir, "m.socket"))
		require.NoError(t, err)
		go msrv.Serve(mlis)
		t.Cleanup(msrv.Stop)
		mconn, err := grpc.NewClient("unix://"+mlis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { mconn.Close() })

		ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "bearer "+good)
		resp, err := environ.NewEnvironClient(mconn).Get(ctx, &environ.GetRequest{})
		require.NoError(t, err)
		require.Empty(t, resp.Environ)
	})
}
