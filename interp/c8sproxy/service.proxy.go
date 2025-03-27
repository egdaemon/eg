package c8sproxy

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"google.golang.org/grpc"
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

func NewServiceProxy(l *log.Logger, ws workspaces.Context, options ...ServiceProxyOption) *ProxyService {
	svc := langx.Clone(ProxyService{
		log: l,
		ws:  ws,
	}, options...)

	return &svc
}

type ProxyService struct {
	c8s.UnimplementedProxyServer
	log           *log.Logger
	ws            workspaces.Context
	cmdenv        []string
	containeropts []string
}

func (t *ProxyService) Bind(host grpc.ServiceRegistrar) {
	c8s.RegisterProxyServer(host, t)
}

func (t *ProxyService) prepcmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.Dir = t.ws.Root
	cmd.Env = t.cmdenv
	cmd.Stdin = os.Stdin
	cmd.Stdout = t.log.Writer()
	cmd.Stderr = t.log.Writer()
	return cmd
}

// Build implements ProxyServer.
func (t *ProxyService) Build(ctx context.Context, req *c8s.BuildRequest) (_ *c8s.BuildResponse, err error) {
	debugx.Println("PROXY CONTAINER BUILD INITIATED", errorsx.Zero(os.Getwd()), t.ws.Root)
	defer debugx.Println("PROXY CONTAINER BUILD COMPLETED", errorsx.Zero(os.Getwd()), t.ws.Root)

	var (
		cmd *exec.Cmd
	)

	abspath := req.Definition
	if !filepath.IsAbs(abspath) {
		abspath = filepath.Join(t.ws.Root, t.ws.WorkingDir, req.Definition)
	}

	// if ok, err := containers.Exists(ctx, req.Name, nil); ok && err == nil {
	// 	return &BuildResponse{}, nil
	// }

	// determine the working directory from the request if specified or the definition file's path.
	wdir := slicesx.FindOrZero(func(s string) bool { return !stringsx.Blank(s) }, req.Directory, filepath.Dir(abspath))
	if cmd, err = PodmanBuild(ctx, req.Name, wdir, abspath, req.Options...); err != nil {
		log.Println("unable to create build command", err)
		return nil, err
	}

	if err = execx.MaybeRun(t.prepcmd(cmd)); err != nil {
		log.Println("unable to exec build command", cmd.String(), err)
		return nil, err
	}

	return &c8s.BuildResponse{}, nil
}

// Pull implements ProxyServer.
func (t *ProxyService) Pull(ctx context.Context, req *c8s.PullRequest) (resp *c8s.PullResponse, err error) {
	debugx.Println("PROXY CONTAINER PULL INITIATED")
	defer debugx.Println("PROXY CONTAINER PULL COMPLETED")

	var (
		cmd *exec.Cmd
	)

	if cmd, err = PodmanPull(ctx, req.Name, req.Options...); err != nil {
		return nil, err
	}

	if err = execx.MaybeRun(t.prepcmd(cmd)); err != nil {
		return nil, err
	}

	return &c8s.PullResponse{}, nil
}

// Run implements ProxyServer.
func (t *ProxyService) Run(ctx context.Context, req *c8s.RunRequest) (_ *c8s.RunResponse, err error) {
	debugx.Println("PROXY CONTAINER RUN INITIATED", errorsx.Zero(os.Getwd()))
	defer debugx.Println("PROXY CONTAINER RUN COMPLETED", errorsx.Zero(os.Getwd()))

	options := append(t.containeropts, req.Options...)
	options = append(
		options,
		"--volume", fmt.Sprintf("%s:%s:rw", eg.DefaultMountRoot(eg.WorkingDirectory), eg.DefaultMountRoot(eg.WorkingDirectory)),
	)

	if err = PodmanRun(ctx, t.prepcmd, req.Image, req.Name, req.Command, options...); err != nil {
		log.Println("failed", req.Image, req.Name, req.Command, strings.Join(options, ", "))
		return nil, err
	}

	return &c8s.RunResponse{}, nil
}

// Module implements ProxyServer.
func (t *ProxyService) Module(ctx context.Context, req *c8s.ModuleRequest) (_ *c8s.ModuleResponse, err error) {
	debugx.Println("PROXY CONTAINER MODULE INITIATED", errorsx.Zero(os.Getwd()))
	defer debugx.Println("PROXY CONTAINER MODULE COMPLETED", errorsx.Zero(os.Getwd()))

	options := make([]string, 0, len(t.containeropts)+len(req.Options)+1)
	options = append(options, t.containeropts...)
	options = append(options, req.Options...)
	options = append(
		options,
		"--volume", fmt.Sprintf("%s:%s:rw", eg.DefaultMountRoot(eg.WorkingDirectory), eg.DefaultMountRoot(eg.WorkingDirectory)),
	)

	if err = PodmanModule(ctx, t.prepcmd, req.Image, req.Name, req.Mdir, options...); err != nil {
		return nil, err
	}

	return &c8s.ModuleResponse{}, nil
}
