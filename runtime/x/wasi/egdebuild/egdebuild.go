// Package egdebuild for building debian packages
package egdebuild

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

const (
	ContainerName = "egdebuild"
)

//go:embed .debian
var debskel embed.FS

type Maintainer struct {
	Name  string
	Email string
}

type ChangeLog struct {
	When time.Time
}

type Config struct {
	Maintainer
	ChangeLog
	SignatureKeyID string   // GPG key ID to use for signing the package.
	Name           string   // name of the package to build. correlates to the DEB_PACKAGE_NAME environment variable.
	Version        string   // version of the package to build. correlates to DEB_VERSION environment variable.
	Distro         string   // distribution to build the package for. correlates to DEB_DISTRO environment variable.
	SourceDir      string   // absolute path to the source files to use for building the package.
	Debian         fs.FS    // debian package files to use for building the package. generally only the rules file needs to be provided. the 'debian' directory is cloned from the fs.FS
	Environ        []string // additional environment variables to pass to the build process.
}

type option func(*Config)

// set the version for the package. if the version string contains :autopatch: then an automatic patch version will be substituted.
// which is useful for uploading to launchpad and other services.
func (option) Version(version string) option {
	return func(c *Config) {
		c.Version = version
	}
}

func (option) Debian(debian fs.FS) option {
	return func(c *Config) {
		c.Debian = debian
	}
}

func (option) ChangeLogDate(ts time.Time) option {
	return func(c *Config) {
		c.ChangeLog.When = ts
	}
}

func (option) Maintainer(name, email string) option {
	return func(c *Config) {
		c.Maintainer.Name = name
		c.Maintainer.Email = email
	}
}

func (option) Distro(s string) option {
	return func(c *Config) {
		c.Distro = s
	}
}

var Option = option(nil)

func From(c Config, opts ...option) Config {
	return langx.Clone(c, opts...)
}

func New(pkg string, distro string, src string, opts ...option) (c Config) {
	return From(Config{
		Name:      pkg,
		Distro:    distro,
		SourceDir: src,
	}, opts...)
}

func Prepare(ctx context.Context, o eg.Op) error {
	relpath := filepath.Join(".debian", "Containerfile")
	if err := egfs.CloneFS(ctx, os.TempDir(), relpath, debskel); err != nil {
		return err
	}

	return eg.Build(Runner().BuildFromFile(filepath.Join(os.TempDir(), relpath)))(ctx, o)
}

func Runner() eg.ContainerRunner {
	return eg.Container(ContainerName)
}

// Build creates a debian package from debian skeleton folder containing.
// requires a working
func Build(cfg Config, opts ...option) eg.OpFn {
	cfg = From(cfg, opts...)
	return func(ctx context.Context, _ eg.Op) error {
		bdir, err := os.MkdirTemp("", "deb.build.*")
		if err != nil {
			return err
		}

		if err := os.MkdirAll(bdir, 0755); err != nil {
			return err
		}

		if err = egfs.CloneFS(ctx, bdir, ".debian", debskel); err != nil {
			return err
		}

		if cfg.Debian != nil {
			if err = egfs.CloneFS(ctx, filepath.Join(bdir, "debian"), "debian", cfg.Debian); err != nil {
				return err
			}
		}

		runtime := shell.Runtime().
			Directory(bdir).
			Environ("DEB_PACKAGE_NAME", cfg.Name).
			Environ("DEB_VERSION", applyversionsubstitutions(cfg)).
			Environ("DEB_DISTRO", cfg.Distro).
			Environ("DEB_CHANGELOG_DATE", cfg.ChangeLog.When.Format(time.RFC1123Z)).
			Environ("DEB_MAINTAINER_EMAIL", cfg.Maintainer.Email).
			Environ("DEB_MAINTAINER_FULLNAME", cfg.Maintainer.Name).
			EnvironFrom(cfg.Environ...)

		return shell.Run(
			ctx,
			runtime.Newf("rsync --verbose --progress --recursive --perms %s/ src/", cfg.SourceDir),
			runtime.New("cat debian/changelog | envsubst | tee debian/changelog"),
			runtime.New("cat debian/control | envsubst | tee debian/control"),
			runtime.New("cat debian/rules | envsubst | tee debian/rules"),
			runtime.Newf("debuild -S -k%s", cfg.SignatureKeyID),
		)
	}
}

func applyversionsubstitutions(cfg Config) string {
	return strings.ReplaceAll(cfg.Version, ":autopatch:", strconv.FormatInt(cfg.ChangeLog.When.Add(dynamicduration(10*time.Second, cfg.Distro)).UnixMilli(), 10))
}

// generate a *consistent* duration based on the input i within the
// provided window. this isn't the best location for these functions.
// but the lack of a better location.
func dynamicduration(window time.Duration, i string) time.Duration {
	if window == 0 {
		return 0
	}

	return time.Duration(backoff.DynamicHashWindow(i, uint64(window)))
}
