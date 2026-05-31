//go:build darwin

package main

import "google.golang.org/grpc"

// controlSocketInterceptors is empty on darwin. The macvm guest has no
// podman, so wrapping every RPC in a podman dial would fail at the very
// first call. Container RPCs aren't valid here regardless — if anything
// tries them they surface their own Unimplemented error.
func controlSocketInterceptors() []grpc.UnaryServerInterceptor {
	return nil
}
