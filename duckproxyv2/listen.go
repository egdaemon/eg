package duckproxyv2

import (
	"context"
	"errors"
	"net"
	"os"
)

// ListenUnix creates a unix socket at path and serves srv on it until ctx
// is cancelled or an error occurs. A stale socket file left over from a
// previous run is removed before binding.
func ListenUnix(ctx context.Context, path string, srv *Server) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer l.Close()

	err = srv.Serve(ctx, l)
	if errors.Is(err, net.ErrClosed) && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
