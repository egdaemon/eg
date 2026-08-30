package podmanx

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"unicode"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"go.podman.io/podman/v6/pkg/bindings"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"google.golang.org/grpc"
)

// podman requires named volumes to match [a-zA-Z0-9][a-zA-Z0-9_.-]*, but forges like
// sourcehut embed characters such as '~' in their repository paths (e.g. ~user/repo).
func volumeNameDisallowed(r rune) bool {
	return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-')
}

// SanitizeVolumeName strips characters that podman disallows in named volumes.
func SanitizeVolumeName(s string) string {
	sanitized, _, err := transform.String(runes.Remove(runes.Predicate(volumeNameDisallowed)), s)
	if err != nil {
		log.Println("sanitization of podman volume name failed", err)
		return s
	}
	return sanitized
}

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
