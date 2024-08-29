package runners

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/workspaces"
	_ "github.com/marcboeker/go-duckdb"
	_ "github.com/shirou/gopsutil/v4/cpu"
	"google.golang.org/grpc"
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
	r := &Agent{
		id:      id,
		ws:      ws,
		blocked: make(chan struct{}),
	}

	for _, opt := range options {
		opt(r)
	}

	go func() {
		log.Println("RUNNER INITIATED", r.id)
		debugx.Println("workspace", spew.Sdump(ws))
		defer log.Println("RUNNER COMPLETED", r.id)
		r.background(ctx)
	}()

	return r, nil
}

type Agent struct {
	id      string
	environ []string
	volumes []string
	ws      workspaces.Context
	blocked chan struct{}
}

func (t Agent) Dial(ctx context.Context) (conn *grpc.ClientConn, err error) {
	cspath := filepath.Join(t.ws.Root, t.ws.RuntimeDir, "control.socket")
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure())
}

func (t Agent) Close() error {
	log.Println("graceful shutdown initiated")
	<-t.blocked
	return nil
}

func (t Agent) background(ctx context.Context) {
	defer close(t.blocked)
	containerOpts := []string{}
	containerOpts = append(containerOpts, t.volumes...)
	containerOpts = append(containerOpts, t.environ...)

	log.Println("CONTAINER OPTIONS", containerOpts)
}
