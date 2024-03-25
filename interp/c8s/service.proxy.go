package c8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/runtime/wasi/langx"
	"github.com/egdaemon/eg/workspaces"
	grpc "google.golang.org/grpc"
)

type ServiceProxyOption func(*ProxyService)

func ServiceProxyOptionCommandEnviron(environ ...string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.cmdenv = environ
	}
}

func ServiceProxyOptionContainerOptions(v ...string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.containeropts = v
	}
}

func NewServiceProxy(ws workspaces.Context, options ...ServiceProxyOption) *ProxyService {
	svc := &ProxyService{
		ws: ws,
	}
	for _, opt := range options {
		opt(svc)
	}

	return svc
}

type ProxyService struct {
	UnimplementedProxyServer
	ws            workspaces.Context
	cmdenv        []string
	containeropts []string
}

func (t *ProxyService) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

func (t *ProxyService) prepcmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.Dir = t.ws.Root
	cmd.Env = t.cmdenv
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Build implements ProxyServer.
func (t *ProxyService) Build(ctx context.Context, req *BuildRequest) (_ *BuildResponse, err error) {
	log.Println("PROXY CONTAINER BUILD INITIATED", langx.Must(os.Getwd()), t.ws.Root)
	defer log.Println("PROXY CONTAINER BUILD COMPLETED", langx.Must(os.Getwd()), t.ws.Root)

	var (
		cmd *exec.Cmd
	)

	abspath := filepath.Join(t.ws.Root, t.ws.WorkingDir, req.Definition)

	if cmd, err = PodmanBuild(ctx, req.Name, req.Directory, abspath, req.Options...); err != nil {
		log.Println("unable to create build command", err)
		return nil, err
	}

	if err = mayberun(t.prepcmd(cmd)); err != nil {
		log.Println("unable to exec build command", err)
		return nil, err
	}

	return &BuildResponse{}, nil
}

// Pull implements ProxyServer.
func (t *ProxyService) Pull(ctx context.Context, req *PullRequest) (resp *PullResponse, err error) {
	debugx.Println("PROXY CONTAINER PULL INITIATED")
	defer debugx.Println("PROXY CONTAINER PULL COMPLETED")

	var (
		cmd *exec.Cmd
	)

	if cmd, err = PodmanPull(ctx, req.Name, req.Options...); err != nil {
		return nil, err
	}

	if err = mayberun(t.prepcmd(cmd)); err != nil {
		return nil, err
	}

	return &PullResponse{}, nil
}

// Run implements ProxyServer.
func (t *ProxyService) Run(ctx context.Context, req *RunRequest) (_ *RunResponse, err error) {
	debugx.Println("PROXY CONTAINER RUN INITIATED", langx.Must(os.Getwd()))
	defer debugx.Println("PROXY CONTAINER RUN COMPLETED", langx.Must(os.Getwd()))

	options := append(
		req.Options,
		t.containeropts...,
	)
	options = append(
		options,
		"--volume", fmt.Sprintf("%s:/opt/eg:O", filepath.Join(t.ws.Root, t.ws.WorkingDir)),
	)

	if err = PodmanRun(ctx, t.prepcmd, req.Image, req.Name, req.Command, options...); err != nil {
		return nil, err
	}

	return &RunResponse{}, nil
}

// Module implements ProxyServer.
func (t *ProxyService) Module(ctx context.Context, req *ModuleRequest) (_ *ModuleResponse, err error) {
	debugx.Println("PROXY CONTAINER MODULE INITIATED", langx.Must(os.Getwd()))
	defer debugx.Println("PROXY CONTAINER MODULE COMPLETED", langx.Must(os.Getwd()))

	options := append(req.Options, t.containeropts...)
	options = append(
		options,
		"--volume", fmt.Sprintf("%s:/opt/egruntime:rw", filepath.Join(t.ws.Root, t.ws.RuntimeDir)),
		"--volume", fmt.Sprintf("%s:/opt/eg:rw", filepath.Join(t.ws.Root, t.ws.WorkingDir)),
	)

	if err = PodmanModule(ctx, t.prepcmd, req.Image, req.Name, req.Mdir, options...); err != nil {
		return nil, err
	}

	return &ModuleResponse{}, nil
}
