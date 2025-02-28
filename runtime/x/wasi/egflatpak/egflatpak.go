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
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Manifests describe how to build the application.
// see https://docs.flatpak.org/en/latest/manifests.html
type Manifest struct {
	ID         string `yaml:"id"`
	Runtime    `yaml:",inline"`
	SDK        `yaml:",inline"`
	Command    string   `yaml:"command"`
	Modules    []Module `yaml:"modules"`
	FinishArgs []string `yaml:"finish-args"`
}

type SDK struct {
	ID      string `yaml:"sdk"`
	Version string `yaml:"-"`
}

type Runtime struct {
	ID      string `yaml:"runtime"`
	Version string `yaml:"runtime-version"`
}

type Source struct {
	Type        string   `yaml:"type"`
	Destination string   `yaml:"dest-filename,omitempty"`
	Path        string   `yaml:"path,omitempty"` // used by directory source.
	Commands    []string `yaml:"commands,omitempty"`
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
		b.Modules = append(
			b.Modules,
			Module{
				Name:        "copy",
				BuildSystem: "simple",
				Commands: []string{
					"cp -r . /app/bin",
				},
				Sources: []Source{
					{Type: "dir", Path: dir},
				},
			})
	})
}

// Specify the runtime to build from. `flatpak list --runtime`
func (t options) Runtime(name, version string) options {
	return append(t, func(b *Builder) {
		b.Runtime = Runtime{ID: name, Version: version}
	})
}

// Specify the sdk to build with.
func (t options) SDK(name, version string) options {
	return append(t, func(b *Builder) {
		b.SDK = SDK{ID: name, Version: version}
	})
}

// Enable gpu access
func (t options) AllowDRI() options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, "--device=dri")
	})
}

// enable access to the wayland socket.
func (t options) AllowWayland() options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, "--socket=wayland")
	})
}

// grant access to the network.
func (t options) AllowNetwork() options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, "--share=network")
	})
}

// grant access to the downloads directory.
func (t options) AllowDownload() options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, "--filesystem=xdg-download")
	})
}

// grant access to the downloads directory.
func (t options) AllowVideos() options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, "--filesystem=xdg-videos")
	})
}

// grant access to the downloads directory.
func (t options) AllowMusic() options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, "--filesystem=xdg-music")
	})
}

// escape hatch for finish-args.
func (t options) Allow(s ...string) options {
	return append(t, func(b *Builder) {
		b.FinishArgs = append(b.FinishArgs, s...)
	})
}

// configure the manifest for building the flatpak, but default it'll copy everything in the current
// directory in an rsync like manner.
func New(id string, command string, options ...option) *Builder {
	return langx.Autoptr(langx.Clone(Builder{
		Manifest: Manifest{
			ID:      id,
			Runtime: Runtime{ID: "org.freedesktop.Platform", Version: "24.08"},
			SDK:     SDK{ID: "org.freedesktop.Sdk", Version: "24.08"},
			Command: command,
			Modules: []Module{},
		},
	}, options...))
}

// Build the flatpak
func Build(ctx context.Context, runtime shell.Command, b *Builder) error {
	var (
		sysdir  = egenv.CacheDirectory(".eg", "flatpak-system")
		userdir = egenv.CacheDirectory(".eg", "flatpak-user")
	)

	if err := egfs.MkDirs(0755, sysdir, userdir); err != nil {
		return err
	}

	dir, err := os.MkdirTemp(".", "flatpak.build.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	manifestpath, err := b.writeManifest(dir)
	if err != nil {
		return err
	}

	runtime = runtime.Environ("FLATPAK_SYSTEM_DIR", sysdir).
		Environ("FLATPAK_USER_DIR", userdir).
		Privileged()

	return shell.Run(
		ctx,
		runtime.Newf("cat %s", manifestpath),
		runtime.New("flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"),
		runtime.Newf("flatpak install --system --assumeyes --include-sdk flathub %s//%s", b.Runtime.ID, b.Runtime.Version),
		runtime.Newf("flatpak install --system --assumeyes flathub %s//%s", b.SDK.ID, b.SDK.Version),
		runtime.Newf("flatpak-builder --system --install-deps-from=flathub --install --force-clean %s %s", dir, manifestpath),
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
