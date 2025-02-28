// Package egflatpak provides utilities for build and publishing software using flatpak.
package egflatpak

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
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
	Modules []Module `yaml:"modules"`
}

type SDK struct {
	ID string `yaml:"sdk"`
}

type Runtime struct {
	ID      string `yaml:"runtime"`
	Version string `yaml:"runtime-version"`
}

type Source struct {
	Type        string   `yaml:"type"`
	Destination string   `yaml:"dest-filename"`
	Commands    []string `yaml:"commands"`
}

type Module struct {
	Name        string   `yaml:"name"`
	BuildSystem string   `yaml:"buildsystem"`
	Commands    []string `yaml:"build-commands"`
	Sources     []Source `yaml:"sources"`
}

type option = func(*Builder)
type options []option

var Option = options(nil)

func (t options) CopyModule(dir string) options {
	return append(t, func(b *Builder) {
		b.Modules = append(b.Modules, Module{Name: "copy", BuildSystem: "simple", Commands: []string{
			"ls -lha .",
			fmt.Sprintf("cp -r %s .", dir),
			"ls -lha .",
			"echo copy done",
		}})
	})
}

// configure the manifest for building the flatpak, but default it'll copy everything in the current
// directory in an rsync like manner.
func New(id string, options ...option) *Builder {
	return langx.Autoptr(langx.Clone(Builder{
		Manifest: Manifest{
			ID:      id,
			Runtime: Runtime{ID: "org.freedesktop.Platform", Version: "23.08"},
			SDK:     SDK{ID: "org.freedesktop.Sdk"},
			Command: "egflatpak.app",
			Modules: []Module{},
		},
	}, options...))
}

// Build the flatpak
func Build(ctx context.Context, runtime shell.Command, b *Builder) error {
	dir, err := os.MkdirTemp(".", "flatpak.build.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	manifestpath, err := b.writeManifest(dir)
	if err != nil {
		return err
	}

	return shell.Run(
		ctx,
		runtime.New("id"),
		runtime.New("chown -R egd:egd .").Privileged(),
		runtime.Newf("chown egd:egd %s", manifestpath).Privileged(),
		runtime.New("tree -L 2 ."),
		runtime.New("pwd"),
		// runtime.New("which flatpak-builder"),
		runtime.Newf("ls -lha %s", manifestpath),
		runtime.Newf("flatpak-builder --force-clean %s %s", dir, manifestpath),
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

func (t Builder) writeManifest(d string) (string, error) {
	encoded, err := yaml.Marshal(t.Manifest)
	if err != nil {
		return "", err
	}

	path := filepath.Join(d, fmt.Sprintf("%s.yml", errorsx.Must(uuid.NewV7())))
	if err = os.WriteFile(path, encoded, 0660); err != nil {
		return "", err
	}

	return path, nil
}
