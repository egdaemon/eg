// Package workspace is responsible for interacting with the workspace file system during the build.
package workspaces

import (
	"context"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/tracex"
	"github.com/gofrs/uuid/v5"
)

func DefaultStateDirectory() string {
	return envx.String(os.TempDir(), "STATE_DIRECTORY", "XDG_STATE_HOME", "EG_STATE_DIRECTORY")
}

type ignorable interface {
	Ignore(string, fs.DirEntry) error // returns an error describing the reason the provided file should be ignored
}

type ignoredir struct {
	path   string
	reason string
}

func (t ignoredir) Ignore(path string, d fs.DirEntry) error {
	if path == t.path {
		return errorsx.String(t.reason)
	}
	return nil
}

type Context struct {
	Module     string // name of the module
	CachedID   string // unique id generated from the content of the module.
	Root       string // workspace root directory.
	WorkingDir string // working directory for modules.
	ModuleDir  string // eg module directory; relative to the root
	CacheDir   string // cache directory. relative to the module directory.
	RuntimeDir string // cache directory for the runner.
	BuildDir   string // directory for built wasm modules; relative to the cache directory.
	TransDir   string // root directory for the transpiled code; relative to the cache directory.
	GenModDir  string // root directory for generated modules; relative to the cache directory.
	Ignore     ignorable
}

func (t Context) FS() fs.FS {
	return os.DirFS(t.Root)
}

func FromEnv(ctx context.Context, root, name string) (zero Context, err error) {
	return Context{
		Module:     name,
		Root:       root,
		RuntimeDir: eg.RuntimeDirectory,
	}, nil
}

type Option func(*Context)

func OptionEnabled(o Option, b bool) Option {
	if !b {
		return OptionNoop
	}

	return o
}

func OptionNoop(ctx *Context) {}
func OptionInvalidateCache(ctx *Context) {
	log.Println("resetting module cache", filepath.Join(ctx.Root, ctx.BuildDir))
	os.RemoveAll(filepath.Join(ctx.Root, ctx.BuildDir))
	os.RemoveAll(filepath.Join(ctx.Root, ctx.TransDir))
}

func New(ctx context.Context, cid hash.Hash, root string, mdir string, name string, private bool, options ...Option) (zero Context, err error) {
	cdir := eg.CacheDirectory
	runtimedir := eg.RuntimeDirectory
	if private {
		runtimedir = filepath.Join(eg.RuntimeDirectory, fmt.Sprintf(".eg.runtime.%x", errorsx.Must(uuid.NewV7()).Bytes()[12:16]))
	}
	ignore := ignoredir{path: cdir, reason: "cache directory"}

	if err = cacheid(ctx, root, mdir, cid, ignore); err != nil {
		return zero, errorsx.Wrap(err, "unable to create cache id")
	}

	_cid := uuid.FromBytesOrNil(cid.Sum(nil)).String()

	return ensuredirs(langx.Clone(Context{
		Module:     name,
		CachedID:   _cid,
		Root:       root,
		ModuleDir:  mdir,
		CacheDir:   cdir,
		RuntimeDir: runtimedir,
		WorkingDir: filepath.Join(runtimedir, "mounted"),
		BuildDir:   filepath.Join(cdir, eg.DefaultModuleDirectory(), ".gen", _cid, "build"),
		TransDir:   filepath.Join(cdir, eg.DefaultModuleDirectory(), ".gen", _cid, "trans"),
		GenModDir:  filepath.Join(cdir, eg.DefaultModuleDirectory(), ".gen", _cid, "trans", ".genmod"),
		Ignore:     ignore,
	}, options...))
}

func ensuredirs(c Context) (_ Context, err error) {
	rdir, err := os.Stat(c.Root)
	if err != nil {
		return c, err
	}

	perms := rdir.Mode() & (fs.ModePerm)

	// need to ensure that certain directories and files exists
	// since they're mounted into containers.
	// the 4 caching/tmp directories are given 0777 permissions
	// because unprivileged users may need to access them.
	err1 := fsx.MkDirs(
		perms,
		filepath.Join(c.Root, c.GenModDir),
		filepath.Join(c.Root, c.BuildDir, c.Module, eg.ModuleDir),
		filepath.Join(c.Root, c.CacheDir),
	)
	tracex.Println("------------ ensuredirs", perms, "------------")

	// need to ensure that certain directories and files exists
	// since they're mounted into containers.
	return c, errorsx.Compact(err1, fsx.MkDirs(
		perms,
		filepath.Join(c.Root, c.RuntimeDir),
		filepath.Join(c.Root, c.WorkingDir),
	))
}

func cacheid(ctx context.Context, root string, mdir string, cacheid hash.Hash, ignore ignorable) error {
	if err := os.MkdirAll(filepath.Join(root, mdir), 0700); err != nil {
		return errorsx.Wrapf(err, "unable to create directory: %s", root)
	}

	return fs.WalkDir(os.DirFS(root), mdir, func(path string, d fs.DirEntry, err error) error {
		var (
			c *os.File
		)

		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if cause := ignore.Ignore(path, d); cause != nil {
			log.Println("skipping", path, cause)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if c, err = os.Open(filepath.Join(root, path)); err != nil {
			return errorsx.WithStack(err)
		}

		if _, err = io.Copy(cacheid, c); err != nil {
			return errorsx.Wrapf(err, "unable to digest file: %s", path)
		}

		return nil
	})
}

func PathRel(tctx Context, mdir string, current string) (path string, err error) {
	if path, err = filepath.Rel(mdir, current); err != nil {
		return "", err
	}
	return filepath.Join(tctx.TransDir, path), nil
}

func PathTranspiled(tctx Context, mdir string, current string) (path string, err error) {
	if path, err = filepath.Rel(mdir, current); err != nil {
		return "", err
	}
	return filepath.Join(tctx.TransDir, path), nil
}

func PathGenMod(tctx Context, mdir string, current string) (path string, err error) {
	if path, err = filepath.Rel(mdir, current); err != nil {
		return "", err
	}
	return filepath.Join(tctx.GenModDir, path), nil
}

func PathBuild(tctx Context, mdir string, current string) (path string, err error) {
	if path, err = filepath.Rel(mdir, current); err != nil {
		return "", err
	}
	return filepath.Join(tctx.BuildDir, path), nil
}

func ReplaceExt(path string, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func TrimRoot(path string, root string) string {
	return strings.TrimPrefix(path, root)
}
