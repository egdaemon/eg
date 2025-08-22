package grpcx

import (
	"context"

	"google.golang.org/grpc"
)

func NoopUnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	return handler(ctx, req)
}
