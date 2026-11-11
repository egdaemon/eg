package cmdopts

import (
	"errors"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/opencontainers/go-digest"
)

// Runtime are the flags picking what a workload's modules run in.
type Runtime struct {
	Runtime string `flag:"" name:"runtime" help:"what the workload's modules run in: a podman container built from the repository's Containerfile, or a vm booted from --egiso" enum:"podman,vm" default:"podman"`
	Egiso   string `flag:"" name:"egiso" help:"path or https URL of the egiso the vm runtime boots"`
	TOC     string `flag:"" name:"toc" help:"digest of the egiso's table of contents"`
}

// Validate the vm runtime boots the egiso named by --egiso and --toc.
func (t Runtime) Validate() error {
	if t.Runtime != eg.RuntimeVM {
		return nil
	}

	if stringsx.Blank(t.Egiso) || stringsx.Blank(t.TOC) {
		return errors.New("--runtime=vm requires --egiso and --toc")
	}

	_, err := digest.Parse(t.TOC)
	return err
}

// Environ is the runtime as the environment a runner reads it from.
func (t Runtime) Environ() []string {
	return []string{
		envx.Format(eg.EnvComputeRuntime, t.Runtime),
		envx.Format(eg.EnvComputeEgiso, t.Egiso),
		envx.Format(eg.EnvComputeEgisoTOC, t.TOC),
	}
}
