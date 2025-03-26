// Package egflatpak provides utilities for build and publishing software using flatpak.
package egflatpak

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"
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
	Type            string   `yaml:"type"`
	Destination     string   `yaml:"dest-filename,omitempty"`    // used by archive source(s).
	Path            string   `yaml:"path,omitempty"`             // used by directory source.
	URL             string   `yaml:"url,omitempty"`              // used by archive source.
	SHA256          string   `yaml:"sha256,omitempty"`           // used by archive source(s).
	StripComponents int      `yaml:"strip-components,omitempty"` // used by archive source(s).
	Tag             string   `yaml:"tag,omitempty"`              // used by git sources(s).
	Commit          string   `yaml:"commit,omitempty"`           // used by git source(s).
	Mirrors         []string `yaml:"mirror-urls,omitempty"`      // used by git source(s).
	Architectures   []string `yaml:"only-arches,omitempty"`      // used by archive source(s).
	Commands        []string `yaml:"commands,omitempty"`
}

type Module struct {
	Name          string   `yaml:"name"`
	BuildSystem   string   `yaml:"buildsystem,omitempty"`
	SubDirectory  string   `yaml:"subdir,omitempty"` // build inside the specified sub directory.
	ConfigOptions []string `yaml:"config-opts,omitempty"`
	Cleanup       []string `yaml:"cleanup,omitempty"`      // files/directories to remove once done.
	PostInstall   []string `yaml:"post-install,omitempty"` // commands to execute post installation.
	Commands      []string `yaml:"build-commands,omitempty"`
	Sources       []Source `yaml:"sources,omitempty"`
}

type option = func(*Builder)
type options []option

func Option() options {
	return options(nil)
}

// assign modules to the builder.
func (t options) Modules(m ...Module) options {
	return append(t, func(b *Builder) {
		b.Modules = m
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

// Build the flatpak, requires that the container is run with --privileged option.
// due to kernel/bwrap issues.
// e.g.)
//
//	func main() {
//		ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
//		defer done()
//		err := eg.Perform(
//			ctx,
//			// not eg.DefaultModule() does not have flatpak installed by default, you'll
//			// have to provide a container with flatpak installed.
//			eg.Build(eg.DefaultModule()),
//			eg.Module(
//				ctx, eg.DefaultModule().OptionLiteral("--privileged"),
//			),
//		)
//		if err != nil {
//			log.Fatalln(err)
//		}
//	}
func Build(ctx context.Context, runtime shell.Command, b *Builder) error {
	var (
		userdir = egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "flatpak-user")
	)

	if err := egfs.MkDirs(0755, userdir); err != nil {
		return err
	}

	dir, err := os.MkdirTemp(".", "flatpak.build.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	manifestpath, err := b.writeManifest(filepath.Join(dir, fmt.Sprintf("%s.yml", errorsx.Must(uuid.NewV7()))))
	if err != nil {
		return err
	}

	// enable ccache
	runtime = runtime.EnvironFrom(
		egccache.Env()...,
	).Environ("FLATPAK_USER_DIR", userdir)

	return shell.Run(
		ctx,
		runtime.New("flatpak --user remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"),
		runtime.Newf("flatpak install --user --assumeyes --include-sdk flathub %s//%s", b.Runtime.ID, b.Runtime.Version),
		runtime.Newf("flatpak install --user --assumeyes flathub %s//%s", b.SDK.ID, b.SDK.Version),
		runtime.Newf("flatpak-builder --user --install-deps-from=flathub --install --ccache --force-clean %s %s", dir, manifestpath),
	)
}

// eg op for running flatpka-builder
func BuildOp(runtime shell.Command, b *Builder) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return Build(ctx, runtime, b)
	}
}

// write the manifest to the specified path.
func ManifestOp(path string, b *Builder) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		_, err := b.writeManifest(path)
		return err
	}
}

type Builder struct {
	Manifest
}

func (t Builder) writeManifest(path string) (string, error) {
	encoded, err := yaml.Marshal(t.Manifest)
	if err != nil {
		return "", err
	}

	if err = os.WriteFile(path, encoded, 0660); err != nil {
		return "", err
	}

	return path, nil
}
