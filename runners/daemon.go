package runners

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
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
	return AgentOptionVolumes(
		AgentMountOverlay(home, home),
		AgentMountOverlay(home, "/root"),
		AgentMountReadOnly(envx.String("", "XDG_RUNTIME_DIR"), envx.String("", "XDG_RUNTIME_DIR")),
	)
}

func AgentOptionAutoEGBin() AgentOption {
	return AgentOptionEGBin(envx.String("", eg.EnvComputeBin))
}

func AgentOptionEGBin(egbin string) AgentOption {
	return AgentOptionVolumes(AgentMountReadOnly(egbin, "/opt/egbin"))
}

func AgentOptionEnviron(environpath string) AgentOption {
	return AgentOptionVolumes(AgentMountReadOnly(environpath, "/opt/egruntime/environ.env"))
}

func AgentOptionVolumeSpecs(desc ...string) []string {
	vs := []string{}
	for _, v := range desc {
		if strings.TrimSpace(v) == "" {
			continue
		}

		vs = append(vs, "--volume", v)
	}

	return vs
}

func AgentOptionVolumes(desc ...string) AgentOption {
	vs := AgentOptionVolumeSpecs(desc...)
	return func(a *Agent) {
		a.volumes = append(a.volumes, vs...)
	}
}

func AgentOptionEnv(key, value string) AgentOption {
	return func(a *Agent) {
		a.environ = append(a.environ, "--env", fmt.Sprintf("%s=%s", key, value))
	}
}

func AgentOptionCores(d uint64) AgentOption {
	return func(a *Agent) {
		a.literals = append(a.literals, "--cpus", strconv.FormatUint(d, 10))
	}
}

func AgentOptionMemory(d uint64) AgentOption {
	return func(a *Agent) {
		a.literals = append(a.literals, "--memory", fmt.Sprintf("%db", d))
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

func AgentOptionCommandLine(literal ...string) AgentOption {
	return func(a *Agent) {
		a.literals = append(a.literals, literal...)
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

func NewRunner(ctx context.Context, ws workspaces.Context, id string, options ...AgentOption) (_ *Agent) {
	r := langx.Clone(Agent{
		id: id,
		ws: ws,
	}, options...)

	return &r
}

type Agent struct {
	id       string
	environ  []string
	volumes  []string
	literals []string
	ws       workspaces.Context
}

func (t Agent) Options() []string {
	containerOpts := []string{}
	containerOpts = append(containerOpts, t.literals...)
	containerOpts = append(containerOpts, t.volumes...)
	containerOpts = append(containerOpts, t.environ...)
	return containerOpts
}

func (t Agent) Dial(ctx context.Context) (conn *grpc.ClientConn, err error) {
	cspath := filepath.Join(t.ws.Root, t.ws.RuntimeDir, "control.socket")
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure())
}
