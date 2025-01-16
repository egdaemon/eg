package cmdopts

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
)

func WithPodman(ctx context.Context) (rctx context.Context, err error) {
	socket := fmt.Sprintf("unix://%s", userx.DefaultRuntimeDirectory("podman", "podman.sock"))
	if rctx, err = bindings.NewConnection(ctx, socket); err != nil {
		return ctx, errorsx.Wrapf(err, "unable to connect to podman: %s", socket)
	}

	return rctx, nil
}
