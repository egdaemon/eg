package runners

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/workspaces"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func DefaultRunnerRuntimeDir() string {
	return filepath.Join("/", "opt", "egruntime")
}

func DefaultRunnerSocketPath() string {
	return filepath.Join(DefaultRunnerRuntimeDir(), "control.socket")
}

type AgentOption func(*Agent)

func AgentOptionNoop(*Agent) {}

func AgentMountSpec(src, dst, mode string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}

	if strings.TrimSpace(dst) == "" {
		return ""
	}

	return fmt.Sprintf("%s:%s:%s", src, dst, stringsx.DefaultIfBlank(mode, "ro"))
}

func AgentMountReadOnly(src, dst string) string {
	return AgentMountSpec(src, dst, "ro")
}

func AgentOptionAutoMountHome(home string) AgentOption {
	return AgentOptionMounts(
		AgentMountSpec(home, "/root", "O"),
		AgentMountReadOnly(envx.String("", "XDG_RUNTIME_DIR"), envx.String("", "XDG_RUNTIME_DIR")),
	)
}

func AgentOptionAutoEGBin() AgentOption {
	return AgentOptionEGBin(envx.String("", "EG_BIN"))
}

func AgentOptionEGBin(egbin string) AgentOption {
	return AgentOptionMounts(AgentMountReadOnly(egbin, "/opt/egbin"))
}

func AgentOptionEnviron(environpath string) AgentOption {
	return AgentOptionMounts(AgentMountReadOnly(environpath, "/opt/egruntime/environ"))
}

func AgentOptionMounts(desc ...string) AgentOption {
	vs := []string{}
	for _, v := range desc {
		if strings.TrimSpace(v) == "" {
			continue
		}

		vs = append(vs, "--volume", v)
	}

	return func(a *Agent) {
		a.volumes = append(a.volumes, vs...)
	}
}

func AgentOptionEnvKeys(keys ...string) AgentOption {
	vs := []string{}
	for _, v := range keys {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}

		vs = append(vs, "--env", v)
	}

	return func(a *Agent) {
		a.environ = append(a.environ, vs...)
	}
}

func DefaultRunnerClient(ctx context.Context) (cc *grpc.ClientConn, err error) {
	daemonpath := DefaultRunnerSocketPath()
	log.Println("connecting", daemonpath)
	if _, err := os.Stat(daemonpath); os.IsNotExist(err) {
		return nil, fmt.Errorf("agent not running at %s", daemonpath)
	}
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", daemonpath), grpc.WithInsecure(), grpc.WithBlock())
}

func NewRunner(ctx context.Context, root string, ws workspaces.Context, id string, options ...AgentOption) (_ *Agent, err error) {
	var (
		control net.Listener
		workdir = filepath.Join(root, id)
		logdst  *os.File
		evtlog  *events.Log
	)

	if err = os.MkdirAll(workdir, 0700); err != nil {
		return nil, errors.WithStack(err)
	}

	if logdst, err = os.Create(filepath.Join(workdir, "daemon.log")); err != nil {
		return nil, errors.WithStack(err)
	}

	if control, err = net.Listen("unix", filepath.Join(workdir, "control.socket")); err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		control.Close()
	}()

	if evtlog, err = events.NewLogEnsureDir(events.NewLogDirFromRunID(root, id)); err != nil {
		return nil, errors.WithStack(err)
	}

	r := &Agent{
		id:      id,
		ws:      ws,
		workdir: workdir,
		control: control,
		evtlog:  evtlog,
		srv: grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
		),
		blocked: make(chan struct{}),
		log:     log.New(logdst, id, log.Flags()),
	}

	for _, opt := range options {
		opt(r)
	}

	go r.background()

	return r, nil
}

type Agent struct {
	id      string
	workdir string
	environ []string
	volumes []string
	ws      workspaces.Context
	control net.Listener
	srv     *grpc.Server
	log     *log.Logger
	blocked chan struct{}
	evtlog  *events.Log
}

func (t Agent) Dial(ctx context.Context) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(t.workdir, "control.socket")), grpc.WithInsecure())
}

func (t Agent) Close() error {
	log.Println("graceful shutdown initiated")
	t.srv.GracefulStop()
	<-t.blocked
	return nil
}

func (t Agent) background() {
	log.Println("RUNNER INITIATED", t.id)
	log.Println("working directory", t.workdir)
	log.Println("workspace", spew.Sdump(t.ws))
	log.Println("control socket", t.control.Addr().String())
	defer close(t.blocked)
	defer log.Println("RUNNER COMPLETED", t.id)
	defer os.RemoveAll(t.workdir)

	events.NewServiceDispatch(t.evtlog).Bind(t.srv)

	containerOpts := []string{}
	containerOpts = append(containerOpts, t.volumes...)
	containerOpts = append(containerOpts, t.environ...)

	c8s.NewServiceProxy(
		t.ws,
		t.workdir,
		c8s.ServiceProxyOptionCommandEnviron(
			append(
				os.Environ(),
				fmt.Sprintf("CI=%s", envx.String("", "EG_CI", "CI")),
				fmt.Sprintf("EG_CI=%s", envx.String("", "EG_CI", "CI")),
				fmt.Sprintf("EG_RUN_ID=%s", t.id),
				fmt.Sprintf("EG_ROOT_DIRECTORY=%s", t.workdir),
			)...,
		),
		c8s.ServiceProxyOptionContainerOptions(
			containerOpts...,
		),
	).Bind(t.srv)

	if err := t.srv.Serve(t.control); err != nil {
		t.log.Println("runner shutdown", err)
		return
	}
}
