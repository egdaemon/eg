package c8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/langx"
	"github.com/egdaemon/eg/workspaces"
	grpc "google.golang.org/grpc"
)

type ServiceProxyOption func(*ProxyService)

func ServiceProxyOptionEnviron(environ ...string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.env = environ
	}
}

func ServiceProxyOptionVolumes(v ...string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.volumes = v
	}
}

func NewServiceProxy(ws workspaces.Context, runtimedir string, options ...ServiceProxyOption) *ProxyService {
	svc := &ProxyService{
		ws:         ws,
		runtimedir: runtimedir,
	}

	for _, opt := range options {
		opt(svc)
	}

	return svc
}

type ProxyService struct {
	UnimplementedProxyServer
	ws         workspaces.Context
	runtimedir string
	env        []string
	volumes    []string
}

func (t *ProxyService) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

func (t *ProxyService) prepcmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.Dir = t.ws.Root
	cmd.Env = t.env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Build implements ProxyServer.
func (t *ProxyService) Build(ctx context.Context, req *BuildRequest) (_ *BuildResponse, err error) {
	debugx.Println("PROXY CONTAINER BUILD INITIATED", langx.Must(os.Getwd()), os.Stdin, os.Stdout, os.Stderr)
	defer debugx.Println("PROXY CONTAINER BUILD COMPLETED", langx.Must(os.Getwd()), os.Stdin, os.Stdout, os.Stderr)

	var (
		cmd *exec.Cmd
	)

	if cmd, err = PodmanBuild(ctx, req.Name, req.Directory, req.Definition, req.Options...); err != nil {
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
		t.volumes...,
	)
	options = append(
		options,
		"--volume", fmt.Sprintf("%s:/opt/eg:O", t.ws.Root),
	)

	if err = PodmanRun(ctx, t.prepcmd, req.Image, req.Name, req.Command, options...); err != nil {
		return nil, err
	}

	return &RunResponse{}, nil
}

// Module implements ProxyServer.
func (t *ProxyService) Module(ctx context.Context, req *ModuleRequest) (_ *ModuleResponse, err error) {
	debugx.Println("PROXY CONTAINER MODULE INITIATED", langx.Must(os.Getwd()), envx.String("eg", "EG_BIN"))
	defer debugx.Println("PROXY CONTAINER MODULE COMPLETED", langx.Must(os.Getwd()), envx.String("eg", "EG_BIN"))

	options := append(
		req.Options,
		"--volume", fmt.Sprintf("%s:/opt/eg:O", t.ws.Root),
		"--volume", fmt.Sprintf("%s:/opt/egruntime", t.runtimedir),
	)

	options = append(options, t.volumes...)

	if err = PodmanModule(ctx, t.prepcmd, req.Image, req.Name, req.Mdir, options...); err != nil {
		log.Println(err)
		return nil, err
	}

	return &ModuleResponse{}, nil
}
