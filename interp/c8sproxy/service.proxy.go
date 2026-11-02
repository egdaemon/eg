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
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"golang.org/x/exp/slices"
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

// ServiceProxyOptionBuildOptions provides options to every container build, they are applied before those of the request.
func ServiceProxyOptionBuildOptions(v ...string) ServiceProxyOption {
	return func(ps *ProxyService) {
		ps.buildopts = v
	}
}

func ServiceProxyOptionBaremetal(ps *ProxyService) {
	ps.remap = func(s string) (n string) {
		old := s
		defer func() {
			debugx.Println("remapped", ps.ws.RuntimeDir, old, "->", n)
		}()

		// baremetal workloads are provided the host directories, which need no remapping.
		if rel, err := filepath.Rel(ps.ws.Root, s); err == nil && filepath.IsLocal(rel) {
			return s
		}

		// the host directories mirror those of the container, resolve the paths of the container within the root.
		// the runtime directory is the one outside of the root directory of the container, see the TODO in egenv.go.
		for _, guest := range []string{eg.DefaultRootDirectory(), eg.DefaultMountRoot()} {
			if rel, err := filepath.Rel(guest, s); err == nil && filepath.IsLocal(rel) {
				return filepath.Join(ps.ws.Root, rel)
			}
		}

		return s
	}
}

func NewServiceProxy(l *log.Logger, ws workspaces.Context, options ...ServiceProxyOption) *ProxyService {
	svc := langx.Clone(ProxyService{
		log:   l,
		ws:    ws,
		remap: func(s string) string { return s }, // noop default
	}, options...)

	return &svc
}

type ProxyService struct {
	c8s.UnimplementedProxyServer
	log           *log.Logger
	ws            workspaces.Context
	remap         func(s string) string
	cmdenv        []string
	containeropts []string
	buildopts     []string
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

// buildoptions are the options of the container build.
func (t *ProxyService) buildoptions(req *c8s.BuildRequest) []string {
	options := make([]string, 0, len(t.buildopts)+len(req.Options))
	options = append(options, t.buildopts...)
	return append(options, req.Options...)
}

// Build implements ProxyServer.
func (t *ProxyService) Build(ctx context.Context, req *c8s.BuildRequest) (_ *c8s.BuildResponse, err error) {
	debugx.Println("PROXY CONTAINER BUILD INITIATED", errorsx.Zero(os.Getwd()), t.ws.Root)
	defer debugx.Println("PROXY CONTAINER BUILD COMPLETED", errorsx.Zero(os.Getwd()), t.ws.Root)
	var (
		cmd *exec.Cmd
	)
	log.Println("DERP DERP")
	abspath := t.remap(req.Definition)
	if !filepath.IsAbs(abspath) {
		abspath = filepath.Join(t.ws.WorkingDir, req.Definition)
	}

	// need to checksum the image.
	// if ok, err := images.Exists(ctx, req.Name, nil); ok && err == nil {
	// 	return &c8s.BuildResponse{}, nil
	// } else {
	// 	debugx.Println("building image", spew.Sdump(req), abspath, spew.Sdump(t.ws))
	// }

	// determine the working directory from the request if specified or the definition file's path.
	wdir := slicesx.FindOrZero(func(s string) bool { return !stringsx.Blank(s) }, req.Directory, filepath.Dir(abspath))

	if cmd, err = podmanx.Build(ctx, req.Name, wdir, abspath, t.buildoptions(req)...); err != nil {
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

	if cmd, err = podmanx.Pull(ctx, req.Name, req.Options...); err != nil {
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

	// log.Println("reqopts", req.Options)
	// log.Println("image", req.Image)
	// log.Println("name", req.Name)
	// log.Println("module", req.Module)
	// log.Println("mdir", req.Mdir)

	{
		// handle the wasi module volume for backwards compatibility.
		idx := slices.IndexFunc(req.Options, func(s string) bool {
			return strings.HasSuffix(s, ":/eg.mnt/.eg.module.wasm:ro")
		})
		if idx > -1 {
			req.Options = slices.Delete(req.Options, idx-1, idx+1)
		}

		path := fsx.LocateFirst(
			filepath.Join(t.ws.Root, t.ws.BuildDir, req.Module),
			eg.DefaultMountRoot(eg.RuntimeDirectory, req.Module),
		)

		// fsx.PrintFS(os.DirFS(path))
		// log.Println("resolved\n", filepath.Join(t.ws.Root, t.ws.BuildDir, req.Module), "\n", eg.DefaultMountRoot(eg.RuntimeDirectory, req.Module), "->", path)
		req.Options = append(req.Options, "--volume", fmt.Sprintf("%s:%s:ro", path, eg.ModuleMount()))
	}

	options := make([]string, 0, len(t.containeropts)+len(req.Options)+1)
	options = append(options, t.containeropts...)
	options = append(options, req.Options...)

	// log.Println("module options", options)
	// envx.Debug(options...)

	if err = PodmanModule(ctx, t.prepcmd, req.Image, req.Name, req.Mdir, options...); err != nil {
		return nil, err
	}

	return &c8s.ModuleResponse{}, nil
}
