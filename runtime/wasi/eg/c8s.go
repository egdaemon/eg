package eg

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegcontainer"
)

// Run the tasks with the specified container.
func Container(name string) C8S {
	return C8S{
		name:  name,
		built: &sync.Once{},
	}
}

type coption []string

func (t coption) workdir(dir string) coption {
	return []string{"-w", dir}
}

func (t coption) envvar(k, v string) coption {
	if v == "" {
		return []string{"-e", k}
	}

	return []string{"-e", fmt.Sprintf("%s=%s", k, v)}
}

func (t coption) literal(options ...string) coption {
	return options
}

// Deprecated: use C8S.
type ContainerRunner = C8S

// C8S is the container implementation of the virtualization layer (C and V).
type C8S struct {
	name       string
	definition string
	pull       string
	cmd        []string
	options    []coption
	built      *sync.Once
}

func (t C8S) Clone() C8S {
	dup := t
	dup.cmd = make([]string, 0, len(t.cmd))
	dup.options = make([]coption, 0, len(t.options))
	copy(dup.cmd, t.cmd)
	copy(dup.options, t.options)
	return t
}

func (t C8S) OptionLiteral(args ...string) C8S {
	t.options = append(t.options, coption(nil).literal(args...))
	return t
}

// specifies the location of the container file on disk.
func (t C8S) BuildFromFile(s string) C8S {
	t.definition = s
	return t
}

// pull the container from a remote repository.
func (t C8S) PullFrom(s string) C8S {
	t.pull = s
	return t
}

// the command to execute.
func (t C8S) Command(s string) C8S {
	t.cmd = strings.Split(s, " ")
	return t
}

// CompileWith builds the container and
func (t C8S) CompileWith(ctx context.Context) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	t.built.Do(func() {
		if t.pull != "" {
			if err = errorsx.Wrapf(ffiegcontainer.Pull(ctx, t.pull, opts), "unable to pull the container: %s", t.name); err != nil {
				return
			}
		}

		if t.definition != "" {
			if err = errorsx.Wrapf(ffiegcontainer.Build(ctx, t.name, t.definition, opts), "unable to build the container: %s", t.name); err != nil {
				return
			}
		}
	})

	return err
}

func (t C8S) RunWith(ctx context.Context, mpath string) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	return errorsx.Wrapf(ffiegcontainer.Run(ctx, t.name, mpath, t.cmd, opts), "unable to run the container: %s", t.name)
}

// internal use.
func (t C8S) ToModuleRunner() ContainerModuleRunner {
	return ContainerModuleRunner{C8S: t}
}

func (t C8S) OptionWorkingDirectory(dir string) C8S {
	t.options = append(t.options, (coption{}).workdir(dir))
	return t
}

func (t C8S) OptionEnvVar(k string) C8S {
	t.options = append(t.options, (coption{}).envvar(k, ""))
	return t
}

func (t C8S) OptionEnv(k, v string) C8S {
	t.options = append(t.options, (coption{}).envvar(k, v))
	return t
}

type ContainerModuleRunner struct {
	C8S
}

func (t ContainerModuleRunner) RunWith(ctx context.Context, mpath string) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	opts = append(opts, "-e", fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1))

	return errorsx.Wrapf(ffiegcontainer.Module(ctx, t.name, mpath, opts), "unable to run the module: %s", t.name)
}
