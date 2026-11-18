package environ_test

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/interp/environ"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestInherited(t *testing.T) {
	t.Run("outside workload", func(t *testing.T) {
		t.Setenv(eg.EnvComputeEnvironToken, "")
		t.Setenv(eg.EnvComputeRuntimeDirectory, "")

		got, err := environ.Inherited(t.Context())
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("within workload", func(t *testing.T) {
		dir := t.TempDir()
		secret := jwtx.JWTSecretSource(func() []byte { return []byte("0123456789abcdef0123456789abcdef") })

		svc := environ.NewService(testx.XFixture("inherited.env"), secret)
		lis, err := net.Listen("unix", filepath.Join(dir, eg.SocketControl))
		require.NoError(t, err)
		srv := grpc.NewServer()
		svc.Bind(srv)
		go srv.Serve(lis)
		t.Cleanup(srv.Stop)

		token, err := environ.NewToken(secret, "run", time.Minute)
		require.NoError(t, err)

		t.Setenv(eg.EnvComputeRuntimeDirectory, dir)
		// inside a workload this is set and takes precedence over the runtime directory,
		// which would dial the real daemon rather than the test server.
		t.Setenv(eg.EnvComputeModuleSocket, filepath.Join(dir, eg.SocketControl))

		t.Run("drops variables the upload determines itself", func(t *testing.T) {
			t.Setenv(eg.EnvComputeEnvironToken, token)

			got, err := environ.Inherited(t.Context())
			require.NoError(t, err)
			require.NotContains(t, got, eg.EnvComputeArch+"=arm64")
			require.Contains(t, got, "FOO=bar")
		})

		// regression: a nested `eg compute upload` inherited the launcher scoped variables of the enclosing
		// workload, notably EG_UNSAFE_GIT_CLONE_ENABLED=false from `eg compute local`, which disabled the
		// clone of the new (remote) workload resulting in an empty /workload/eg.
		t.Run("drops launcher scoped variables", func(t *testing.T) {
			t.Setenv(eg.EnvComputeEnvironToken, token)

			got, err := environ.Inherited(t.Context())
			require.NoError(t, err)
			require.Equal(t, []string{"FOO=bar"}, got)
		})

		t.Run("invalid token", func(t *testing.T) {
			t.Setenv(eg.EnvComputeEnvironToken, "garbage")

			_, err := environ.Inherited(t.Context())
			require.Error(t, err)
		})
	})
}
