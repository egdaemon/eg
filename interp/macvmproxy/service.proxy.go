// Package macvmproxy serves the eg.interp.macvm.Proxy RPC service. On
// darwin/arm64 it shells out to the `tart` CLI (cirruslabs/tart) for the
// full VM lifecycle. Other host platforms receive Unimplemented errors.
package macvmproxy

import (
	"context"
	"log"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/interp/macvm"
	"github.com/egdaemon/eg/workspaces"
	"google.golang.org/grpc"
)

type ServiceProxyOption func(*ProxyService)

// ServiceProxyOptionEGBin sets the path to the host eg binary so the proxy
// can share it into the guest for nested Module execution.
func ServiceProxyOptionEGBin(path string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.egbin = path
	}
}

func NewServiceProxy(l *log.Logger, ws workspaces.Context, options ...ServiceProxyOption) *ProxyService {
	svc := langx.Clone(ProxyService{
		log: l,
		ws:  ws,
	}, options...)
	return &svc
}

type ProxyService struct {
	macvm.UnimplementedProxyServer
	log   *log.Logger
	ws    workspaces.Context
	egbin string
}

func (t *ProxyService) Bind(host grpc.ServiceRegistrar) {
	macvm.RegisterProxyServer(host, t)
}

// Shutdown stops every guest the proxy started. Tart preserves the cloned VM
// state across `tart stop`, so subsequent Pulls short-circuit on the existing
// VM and any in-guest caches (brew, build state) survive.
func (t *ProxyService) Shutdown(ctx context.Context) error {
	return shutdown(ctx)
}
