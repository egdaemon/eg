package environ

import (
	"bytes"
	"context"
	"net"
	"os"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"google.golang.org/grpc/metadata"
)

// Inherited retrieves the environment the enclosing workload was provided from the control socket.
// nil when not running within a workload, i.e. no token or runtime directory is available.
// the variables that an upload determines itself are dropped.
func Inherited(ctx context.Context) ([]string, error) {
	token := os.Getenv(eg.EnvComputeEnvironToken)
	rtdir := os.Getenv(eg.EnvComputeRuntimeDirectory)
	if token == "" || rtdir == "" {
		return nil, nil
	}

	conn, err := egunsafe.NewControlSocket(&net.Dialer{}).DialContext(ctx)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to dial the control socket")
	}
	defer conn.Close()

	// the jwt library only recognizes the lowercase scheme.
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+token)
	resp, err := NewEnvironClient(conn).Get(ctx, &GetRequest{})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve the workload environment")
	}

	return envx.Build().
		FromReader(bytes.NewReader(resp.Environ)).
		Drop(
			eg.EnvComputeArch,
			eg.EnvComputeOS,
			eg.EnvWorkloadDraft,
			eg.EnvWorkloadRelease,
			eg.EnvGCPWorkloadIdentityEnabled,
		).Environ()
}
