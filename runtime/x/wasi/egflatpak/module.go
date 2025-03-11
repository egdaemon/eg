package egflatpak

import (
	"github.com/egdaemon/eg/internal/langx"
)

type moption = func(*Module)
type moptions []moption

func NewModule(name, system string, options ...moption) Module {
	return langx.Clone(Module{
		Name:        name,
		BuildSystem: system,
	}, options...)
}

func ModuleOptions(options ...moption) moptions {
	return moptions(options)
}

// shell commands to execute during the build
func (t moptions) Commands(cmds ...string) moptions {
	return append(t, func(m *Module) {
		m.Commands = cmds
	})
}

// sources to pull
func (t moptions) Sources(sources ...Source) moptions {
	return append(t, func(m *Module) {
		m.Sources = sources
	})
}

// configuration options for the build system.
func (t moptions) ConfigOptions(options ...string) moptions {
	return append(t, func(m *Module) {
		m.ConfigOptions = options
	})
}

// directories to remove once done.
func (t moptions) Cleanup(dirs ...string) moptions {
	return append(t, func(m *Module) {
		m.Cleanup = dirs
	})
}

// Not recommended, here for testing: build a module directly from a directory.
func ModuleCopy(dir string) Module {
	return NewModule("copy", "simple", ModuleOptions().Commands(
		"cp -r . /app/bin",
	).Sources(SourceDir(dir))...)
}

// build a module from a binary tarball.
func ModuleTarball(url, sha256d string) Module {
	return NewModule("tarball", "simple", ModuleOptions().Commands(
		"cp -r . /app/bin",
	).Sources(SourceTarball(url, sha256d))...)
}
