package podmanx

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"go.podman.io/podman/v6/pkg/bindings"
	"google.golang.org/grpc"
)

func WithClient(ctx context.Context) (rctx context.Context, err error) {
	socket := envx.String(DefaultSocket(), eg.EnvPodmanSocket)
	debugx.Println("podman socket", socket)
	if rctx, err = bindings.NewConnection(ctx, socket); err != nil {
		return ctx, errorsx.Wrapf(err, "unable to connect to podman: %s", socket)
	}

	return rctx, nil
}

// Create a unary server interceptor that adds the root context to the request context
func GrpcClient(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	pctx, err := bindings.NewConnection(ctx, DefaultSocket())
	if err != nil {
		return nil, err
	}

	// Call the next handler with the new context
	return handler(pctx, req)
}

// generates the platform architecture string defaulting to
// host os/arch when not provided.
func AutoPlatform(arch string, os string) string {
	return fmt.Sprintf("%s/%s", langx.FirstNonZero(os, runtime.GOOS), langx.FirstNonZero(arch, runtime.GOARCH))
}

func Build(ctx context.Context, name string, dir string, definition string, options ...string) (cmd *exec.Cmd, err error) {
	args := []string{
		"build", "--stdin", "-t", name, "-f", definition,
	}
	args = append(args, options...)
	args = append(args, dir)

	return exec.CommandContext(ctx, "podman", args...), nil
}

func Pull(ctx context.Context, name string, options ...string) (cmd *exec.Cmd, err error) {
	args := []string{
		"pull", name,
	}
	args = append(args, options...)

	return exec.CommandContext(ctx, "podman", args...), nil
}
