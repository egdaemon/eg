// Package egflatpak provides utilities for build and publishing software using flatpak.
package egflatpak

import (
	"context"
	"fmt"
	"os"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Manifests describe how to build the application.
// see https://docs.flatpak.org/en/latest/manifests.html
type Manifest struct {
	ID      string `yaml:"id"`
	Runtime `yaml:",inline"`
	SDK     `yaml:",inline"`
	Command string   `yaml:"command"`
	Modules []Module `yaml:"module"`
}

type SDK struct {
	ID string `yaml:"sdk"`
}

type Runtime struct {
	ID      string `yaml:"runtime"`
	Version string `yaml:"runtime-version"`
}

type Module struct {
	Name     string   `yaml:"name"`
	System   string   `yaml:"buildsystem"`
	Commands []string `yaml:"commands"`
}

type options []option

var Option = options(nil)

func (t options) Runtime(id, version string) options {
	return append(t, func(b *Builder) {
		b.Runtime = Runtime{ID: id, Version: version}
	})
}

type option = func(*Builder)

func New(id string, options ...option) *Builder {
	return langx.Autoptr(langx.Clone(Builder{
		Manifest: Manifest{
			ID:      id,
			Runtime: Runtime{ID: "org.freedesktop.Platform", Version: "23.08"},
			SDK:     SDK{ID: "org.freedesktop.Sdk"},
			Command: "egflatpak.app",
		},
	}, options...))
}

func Build(ctx context.Context, runtime shell.Command, b *Builder) error {
	manifestpath, err := b.writeManifest()
	if err != nil {
		return err
	}

	return shell.Run(
		ctx,
		runtime.New("which flatpak-build"),
		runtime.Newf("flatpak-builder . %s", manifestpath),
	)
}

func BuildOp(runtime shell.Command, b *Builder) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return Build(ctx, runtime, b)
	}
}

type Builder struct {
	Manifest
}

func (t Builder) writeManifest() (string, error) {
	encoded, err := yaml.Marshal(t.Manifest)
	if err != nil {
		return "", err
	}

	path := egenv.EphemeralDirectory(fmt.Sprintf("%s.yml", errorsx.Must(uuid.NewV7())))
	if err = os.WriteFile(path, encoded, 0600); err != nil {
		return "", err
	}

	return path, nil
}
