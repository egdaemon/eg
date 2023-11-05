package c8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	grpc "google.golang.org/grpc"
)

type ServiceProxyOption func(*ProxyService)

func ServiceProxyOptionEnviron(environ ...string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.env = environ
	}
}

func NewServiceProxy(root string, options ...ServiceProxyOption) *ProxyService {
	svc := &ProxyService{
		root: root,
	}

	for _, opt := range options {
		opt(svc)
	}

	return svc
}

type ProxyService struct {
	UnimplementedProxyServer
	root string
	env  []string
}

func (t *ProxyService) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

// Build implements ProxyServer.
func (t *ProxyService) Build(ctx context.Context, req *BuildRequest) (_ *BuildResponse, err error) {
	log.Println("PROXY CONTAINER BUILD INITIATED")
	defer log.Println("PROXY CONTAINER BUILD COMPLETED")

	var (
		cmd *exec.Cmd
	)

	if cmd, err = PodmanBuild(ctx, req.Name, req.Directory, req.Definition, req.Options...); err != nil {
		return nil, err
	}

	if err = mayberun(cmd); err != nil {
		return nil, err
	}

	return &BuildResponse{}, nil
}

// Pull implements ProxyServer.
func (t *ProxyService) Pull(ctx context.Context, req *PullRequest) (resp *PullResponse, err error) {
	log.Println("PROXY CONTAINER PULL INITIATED")
	defer log.Println("PROXY CONTAINER PULL COMPLETED")

	var (
		cmd *exec.Cmd
	)

	if cmd, err = PodmanPull(ctx, req.Name, req.Options...); err != nil {
		return nil, err
	}

	if err = mayberun(cmd); err != nil {
		return nil, err
	}

	return &PullResponse{}, nil
}

// Run implements ProxyServer.
func (t *ProxyService) Run(ctx context.Context, req *RunRequest) (_ *RunResponse, err error) {
	log.Println("PROXY CONTAINER RUN INITIATED")
	defer log.Println("PROXY CONTAINER RUN COMPLETED")

	cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Dir = t.root
		cmd.Env = t.env
		cmd.Stdin = os.Stdin
		// cmd.Stdout = os.Stdout
		// cmd.Stderr = os.Stderr
		return cmd
	}

	options := append(
		req.Options,
		"--volume", fmt.Sprintf("%s:/opt/eg:O", t.root),
	)

	if err = PodmanRun(ctx, cmdctx, req.Image, req.Name, req.Command, options...); err != nil {
		return nil, err
	}

	return &RunResponse{}, nil
}

// Module implements ProxyServer.
func (t *ProxyService) Module(ctx context.Context, req *ModuleRequest) (_ *ModuleResponse, err error) {
	log.Println("PROXY CONTAINER MODULE INITIATED")
	defer log.Println("PROXY CONTAINER MODULE COMPLETED")

	cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Dir = t.root
		cmd.Env = t.env
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd
	}

	if err = PodmanModule(ctx, cmdctx, req.Image, req.Name, req.Mdir, req.Options...); err != nil {
		return nil, err
	}

	return &ModuleResponse{}, nil
}
