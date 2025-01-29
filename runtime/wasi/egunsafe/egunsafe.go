package egunsafe

import (
	"context"
	"path/filepath"

	"github.com/egdaemon/eg"
	"google.golang.org/grpc"
)

func DialControlSocket(ctx context.Context) (conn *grpc.ClientConn, err error) {
	// cspath := RuntimeDirectory(eg.SocketControl)
	// return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure())
	return grpc.DialContext(ctx, "localhost:15999", grpc.WithInsecure())

}

func RuntimeDirectory(paths ...string) string {
	return eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Join(paths...))
}
