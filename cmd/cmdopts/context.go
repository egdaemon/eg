package cmdopts

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
)

func WithPodman(ctx context.Context) (rctx context.Context, err error) {
	socket := fmt.Sprintf("unix://%s", userx.DefaultRuntimeDirectory("..", "podman", "podman.sock"))
	u := userx.CurrentUserOrDefault(userx.Root())
	if u.Uid != "0" {
		socket = fmt.Sprintf("unix:///var/run/user/%s/podman/podman.sock", u.Uid)
	}

	if rctx, err = bindings.NewConnection(ctx, envx.String(socket, eg.EnvPodmanSocket)); err != nil {
		return ctx, errorsx.Wrapf(err, "unable to connect to podman: %s", socket)
	}

	return rctx, nil
}
