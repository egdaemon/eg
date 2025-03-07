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

func DefaultRunnerSocketPath() string {
	return eg.DefaultMountRoot(eg.RuntimeDirectory, eg.SocketControl)
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
	return AgentOptionVolumes(AgentMountReadOnly(egbin, eg.DefaultMountRoot(eg.BinaryBin)))
}

func AgentOptionEnviron(environpath string) AgentOption {
	return AgentOptionCompose(
		AgentOptionVolumes(AgentMountReadOnly(environpath, eg.DefaultMountRoot(eg.RuntimeDirectory, eg.EnvironFile))),
		AgentOptionCommandLine("--env-file", environpath),
	)
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

func AgentOptionContainerCache(dir string) string {
	return AgentMountReadWrite(dir, "/var/lib/containers")
}

// standard caching mounts across host environments for local compute, let podman deal with the issues.
// since they cant seem to figure out how to make host direcctory mounts function identically.
func AgentOptionLocalComputeCachingVolumes(canonicaluri string) AgentOption {
	_, path, _ := strings.Cut(canonicaluri, ":")
	path = strings.ReplaceAll(path, "/", ".")
	path = strings.ReplaceAll(path, ".git", "")
	return AgentOptionCompose(
		AgentOptionVolumes(
			AgentMountReadWrite(fmt.Sprintf("%s.eg.containers", path), "/var/lib/containers"),
			AgentMountReadWrite(fmt.Sprintf("%s.eg.cache", path), eg.DefaultMountRoot(eg.CacheDirectory)),
		),
	)
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

func AgentOptionAutoRemote() AgentOption {
	if host := envx.String("", eg.EnvContainerHost); stringsx.Present(host) {
		return AgentOptionCompose(
			AgentOptionEnv(eg.EnvContainerHost, eg.DefaultRuntimeDirectory("podman.socket")),
			AgentOptionVolumes(
				AgentMountReadWrite(
					strings.TrimPrefix(host, "unix://"),
					eg.DefaultRuntimeDirectory("podman.socket"),
				),
			),
		)
	} else {
		log.Println("container host not present", host)
	}

	return AgentOptionNoop
}

func AgentOptionCompose(options ...AgentOption) AgentOption {
	return func(a *Agent) {
		for _, opt := range options {
			opt(a)
		}
	}
}

func AgentOptionCommandLine(literal ...string) AgentOption {
	return func(a *Agent) {
		a.literals = append(a.literals, literal...)
	}
}

// Only should be used for local compute.
func AgentOptionPublish(ports ...int) AgentOption {
	return func(a *Agent) {
		for _, p := range ports {
			a.literals = append(a.literals, "--publish", fmt.Sprintf("%d:%d", p, p))
		}
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
	cspath := filepath.Join(t.ws.Root, t.ws.RuntimeDir, eg.SocketControl)
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithInsecure())
}
