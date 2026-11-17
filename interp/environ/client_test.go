package environ_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/interp/environ"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestInheritedEnvironOutsideWorkload(t *testing.T) {
	t.Setenv(eg.EnvComputeEnvironToken, "")
	t.Setenv(eg.EnvComputeRuntimeDirectory, "")

	got, err := environ.Inherited(t.Context())
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestInheritedEnviron(t *testing.T) {
	dir := t.TempDir()
	secret := jwtx.JWTSecretSource(func() []byte { return []byte("0123456789abcdef0123456789abcdef") })
	require.NoError(t, os.WriteFile(filepath.Join(dir, eg.EnvironFile), []byte("FOO=bar\n"+eg.EnvComputeArch+"=arm64\n"), 0600))

	svc := environ.NewService(filepath.Join(dir, eg.EnvironFile), secret)
	lis, err := net.Listen("unix", filepath.Join(dir, eg.SocketControl))
	require.NoError(t, err)
	srv := grpc.NewServer()
	svc.Bind(srv)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	token, err := environ.NewToken(secret, "run", time.Minute)
	require.NoError(t, err)

	t.Setenv(eg.EnvComputeRuntimeDirectory, dir)
	t.Setenv(eg.EnvComputeEnvironToken, token)

	got, err := environ.Inherited(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"FOO=bar"}, got)

	t.Setenv(eg.EnvComputeEnvironToken, "garbage")
	_, err = environ.Inherited(t.Context())
	require.Error(t, err)
}
