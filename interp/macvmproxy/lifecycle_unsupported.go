//go:build !(darwin && arm64)

package macvmproxy

import (
	"context"

	"github.com/egdaemon/eg/interp/macvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (t *ProxyService) Pull(ctx context.Context, req *macvm.PullRequest) (*macvm.PullResponse, error) {
	return nil, status.Error(codes.Unimplemented, "macvm requires darwin/arm64 host (Apple Silicon)")
}

func (t *ProxyService) Run(ctx context.Context, req *macvm.RunRequest) (*macvm.RunResponse, error) {
	return nil, status.Error(codes.Unimplemented, "macvm requires darwin/arm64 host (Apple Silicon)")
}

func (t *ProxyService) Module(ctx context.Context, req *macvm.ModuleRequest) (*macvm.ModuleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "macvm requires darwin/arm64 host (Apple Silicon)")
}

func shutdown(_ context.Context) error {
	return nil
}
