package cmdopts

import (
	"context"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/podmanx"
)

func WithPodman(ctx context.Context) (rctx context.Context, err error) {
	socket := envx.String(podmanx.DefaultSocket(), eg.EnvPodmanSocket)
	if rctx, err = bindings.NewConnection(ctx, socket); err != nil {
		return ctx, errorsx.Wrapf(err, "unable to connect to podman: %s", socket)
	}
	return rctx, nil
}
