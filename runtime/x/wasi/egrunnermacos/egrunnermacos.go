// Package egrunnermacos runs eg modules inside a macOS guest VM via the
// host-side macvm proxy, which shells out to the Tart CLI. The user-facing
// surface mirrors runtime/wasi/eg.ContainerRunner: declare a Runner with
// PullFrom, then drive it through eg.Build / eg.Module just like a container.
package egrunnermacos

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegmacvm"
)

type option []string

func (option) workdir(dir string) option {
	return []string{"-w", dir}
}

func (option) envvar(k, v string) option {
	if v == "" {
		return []string{"-e", k}
	}
	return []string{"-e", fmt.Sprintf("%s=%s", k, v)}
}

func (option) literal(args ...string) option {
	return args
}

// New returns a Runner that will boot a macOS VM identified by name.
func New(name string) Runner {
	return Runner{
		name:  name,
		built: &sync.Once{},
	}
}

// Runner declares a macOS VM and the command/module it will host.
type Runner struct {
	name    string
	image   string
	cmd     []string
	options []option
	built   *sync.Once
}

// PullFrom pulls a Tart-format macOS image from an OCI registry. Compatible
// images live at e.g. ghcr.io/cirruslabs/macos-sequoia-base:latest.
func (t Runner) PullFrom(image string) Runner {
	t.image = image
	return t
}

// Command is the shell command to execute inside the guest.
func (t Runner) Command(s string) Runner {
	t.cmd = strings.Split(s, " ")
	return t
}

func (t Runner) OptionLiteral(args ...string) Runner {
	t.options = append(t.options, option{}.literal(args...))
	return t
}

func (t Runner) OptionWorkingDirectory(dir string) Runner {
	t.options = append(t.options, option{}.workdir(dir))
	return t
}

func (t Runner) OptionEnvVar(k string) Runner {
	t.options = append(t.options, option{}.envvar(k, ""))
	return t
}

func (t Runner) OptionEnv(k, v string) Runner {
	t.options = append(t.options, option{}.envvar(k, v))
	return t
}

func (t Runner) CompileWith(ctx context.Context) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	t.built.Do(func() {
		if t.image != "" {
			err = errorsx.Wrapf(ffiegmacvm.Pull(ctx, t.name, t.image, opts), "unable to pull macvm image: %s", t.name)
		}
	})

	return err
}

func (t Runner) RunWith(ctx context.Context, _ string) error {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	return errorsx.Wrapf(ffiegmacvm.Run(ctx, t.name, t.cmd, opts), "unable to run macvm: %s", t.name)
}

// ToModuleRunner returns a Runner variant that dispatches Module rather than Run.
func (t Runner) ToModuleRunner() ModuleRunner {
	return ModuleRunner{Runner: t}
}

// ModuleRunner runs the workload as a nested eg module inside the guest.
type ModuleRunner struct {
	Runner
}

func (t ModuleRunner) RunWith(ctx context.Context, mpath string) error {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	opts = append(opts, "-e", fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1))

	return errorsx.Wrapf(ffiegmacvm.Module(ctx, t.name, mpath, opts), "unable to run macvm module: %s", t.name)
}
