//go:build !darwin

package main

import (
	"github.com/egdaemon/eg/internal/podmanx"
	"google.golang.org/grpc"
)

// controlSocketInterceptors injects the podman client connection into every
// incoming RPC on the in-guest control socket. The c8s proxy depends on a
// live podman context per request; on Linux containers podman is always
// reachable via its UDS so the connection is cheap and never fails.
func controlSocketInterceptors() []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{podmanx.GrpcClient}
}
