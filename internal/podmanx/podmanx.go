package podmanx

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
	"google.golang.org/grpc"
)

func DefaultSocket() string {
	return fmt.Sprintf("unix://%s", userx.DefaultRuntimeDirectory("podman", "podman.sock"))
}

func WithClient(ctx context.Context) (rctx context.Context, err error) {
	socket := DefaultSocket()
	if rctx, err = bindings.NewConnection(ctx, socket); err != nil {
		return ctx, errorsx.Wrapf(err, "unable to connect to podman: %s", socket)
	}

	return rctx, nil
}

// Create a unary server interceptor that adds the root context to the request context
func GrpcClient(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	pctx, err := bindings.NewConnection(ctx, DefaultSocket())
	if err != nil {
		return nil, err
	}

	// Call the next handler with the new context
	return handler(pctx, req)
}
