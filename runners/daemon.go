package runners

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/workspaces"
	_ "github.com/marcboeker/go-duckdb"
	_ "github.com/shirou/gopsutil/v4/cpu"
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

func AgentMountReadWrite(src, dst string) string {
	return AgentMountSpec(src, dst, "rw")
}

// Mounts a path using a overlay FS making it mutable within the container
// but those changes don't persist.
func AgentMountOverlay(src, dst string) string {
	return AgentMountSpec(src, dst, "O")
}

func AgentOptionAutoMountHome(home string) AgentOption {
	return AgentOptionMounts(
		AgentMountOverlay(home, home),
		AgentMountOverlay(home, "/root"),
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
	return AgentOptionMounts(AgentMountReadOnly(environpath, "/opt/egruntime/environ.env"))
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

func AgentOptionEnv(key, value string) AgentOption {
	return func(a *Agent) {
		a.environ = append(a.environ, "--env", fmt.Sprintf("%s=%s", key, value))
	}
}

func AgentOptionEnvKeys(keys ...string) AgentOption {
	vs := []string{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k == "" {
			continue
		}

		vs = append(vs, "--env", k)
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

func NewRunner(ctx context.Context, ws workspaces.Context, id string, options ...AgentOption) (_ *Agent, err error) {
	var (
		control  net.Listener
		logdst   *os.File
		evtlog   *events.Log
		db       *sql.DB
		cspath   = filepath.Join(ws.Root, ws.RuntimeDir, "control.socket")
		logpath  = filepath.Join(ws.Root, ws.RuntimeDir, "daemon.log")
		eventsdb = filepath.Join(ws.Root, ws.RuntimeDir, "analytics.db")
	)

	if logdst, err = os.Create(logpath); err != nil {
		return nil, errorsx.WithStack(err)
	}

	if evtlog, err = events.NewLogEnsureDir(events.NewLogDirFromRunID(ws.Root, ws.RuntimeDir)); err != nil {
		return nil, errorsx.WithStack(err)
	}

	if db, err = sql.Open("duckdb", eventsdb); err != nil {
		return nil, errorsx.Wrap(err, "unable to create analytics.db")
	}

	if err = events.PrepareMetrics(ctx, db); err != nil {
		return nil, errorsx.Wrap(err, "unable to prepare analytics.db")
	}

	if control, err = net.Listen("unix", cspath); err != nil {
		return nil, errorsx.Wrap(err, "unable to create control.socket")
	}

	go func() {
		<-ctx.Done()
		control.Close()
	}()

	r := &Agent{
		id:          id,
		ws:          ws,
		control:     control,
		evtlog:      evtlog,
		analyticsdb: db,
		srv: grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
		),
		blocked: make(chan struct{}),
		log:     log.New(io.MultiWriter(os.Stderr, logdst), id, log.Flags()),
	}

	for _, opt := range options {
		opt(r)
	}

	go func() {
		log.Println("RUNNER INITIATED", r.id)
		log.Println("control socket", control.Addr().String())
		log.Println("analytics database", eventsdb)
		debugx.Println("workspace", spew.Sdump(ws))
		defer log.Println("RUNNER COMPLETED", r.id)
		r.background(ctx)
	}()

	return r, nil
}

type Agent struct {
	id          string
	environ     []string
	volumes     []string
	ws          workspaces.Context
	analyticsdb *sql.DB
	control     net.Listener
	srv         *grpc.Server
	log         *log.Logger
	blocked     chan struct{}
	evtlog      *events.Log
}

func (t Agent) Dial(ctx context.Context) (conn *grpc.ClientConn, err error) {
	cspath := filepath.Join(t.ws.Root, t.ws.RuntimeDir, "control.socket")
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure())
}

func (t Agent) Close() error {
	log.Println("graceful shutdown initiated")
	t.srv.GracefulStop()
	<-t.blocked
	return nil
}

func (t Agent) background(ctx context.Context) {
	defer close(t.blocked)
	defer t.analyticsdb.Close()

	// periodic sampling of system metrics
	go systemload(ctx, t.analyticsdb)
	// final sample
	defer func() {
		errorsx.Log(systemloadsample(ctx, t.analyticsdb))
	}()

	events.NewServiceDispatch(t.evtlog, t.analyticsdb).Bind(t.srv)

	containerOpts := []string{}
	containerOpts = append(containerOpts, t.volumes...)
	containerOpts = append(containerOpts, t.environ...)

	c8s.NewServiceProxy(
		t.log,
		t.ws,
		c8s.ServiceProxyOptionCommandEnviron(
			errorsx.Zero(
				envx.Build().FromEnv("PATH", "TERM", "COLORTERM", "LANG").Var(
					"CI", envx.String("", "EG_CI", "CI"),
				).Var(
					"EG_CI", envx.String("", "EG_CI", "CI"),
				).Var(
					"EG_RUN_ID", t.id,
				).Environ(),
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
