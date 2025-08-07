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

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
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
	Module      string // name of the module
	CachedID    string // unique id generated from the content of the module.
	Root        string // workspace root directory.
	WorkingDir  string // working directory for modules.
	ModuleDir   string // eg module directory; relative to the root
	CacheDir    string // cache directory. relative to the module directory.
	RuntimeDir  string // eg internal directory
	WorkloadDir string // shared directory between modules within a single workload.
	BuildDir    string // directory for built wasm modules; relative to the cache directory.
	TransDir    string // root directory for the transpiled code; relative to the cache directory.
	GenModDir   string // root directory for generated modules; relative to the cache directory.
	Ignore      ignorable
}

func (t Context) FS() fs.FS {
	return os.DirFS(t.Root)
}

// 14:18:01 interp.go:227: workspace (workspaces.Context) {
//  Module: (string) (len=15) "tests/baremetal",
//  CachedID: (string) (len=36) "64b892ee-de11-5e8d-61db-8e7f4ff28c21",
//  Root: (string) (len=31) "/home/jatone/development/egd/eg",
//  WorkingDir: (string) (len=40) ".eg.runtime/.eg.runtime.941b9af4/mounted",
//  ModuleDir: (string) (len=3) ".eg",
//  CacheDir: (string) (len=9) ".eg.cache",
//  RuntimeDir: (string) (len=32) ".eg.runtime/.eg.runtime.941b9af4",
//  WorkloadDir: (string) (len=45) ".eg.runtime/.eg.runtime.941b9af4/.eg.workload",
//  BuildDir: (string) (len=61) ".eg.cache/.eg/.gen/64b892ee-de11-5e8d-61db-8e7f4ff28c21/build",
//  TransDir: (string) (len=61) ".eg.cache/.eg/.gen/64b892ee-de11-5e8d-61db-8e7f4ff28c21/trans",
//  GenModDir: (string) (len=69) ".eg.cache/.eg/.gen/64b892ee-de11-5e8d-61db-8e7f4ff28c21/trans/.genmod",
//  Ignore: (workspaces.ignoredir) {
//   path: (string) (len=9) ".eg.cache",
//   reason: (string) (len=15) "cache directory"
//  }
// }

// [25]14:18:06 workspace.go:68: WAKA (workspaces.Context) {
//  Module: (string) (len=23) "/eg.mnt/.eg.module.wasm",
//  CachedID: (string) "",
//  Root: (string) (len=9) "/workload",
//  WorkingDir: (string) "",
//  ModuleDir: (string) "",
//  CacheDir: (string) "",
//  RuntimeDir: (string) (len=11) ".eg.runtime",
//  WorkloadDir: (string) (len=24) ".eg.runtime/.eg.workload",
//  BuildDir: (string) "",
//  TransDir: (string) "",
//  GenModDir: (string) "",
//  Ignore: (workspaces.ignorable) <nil>
// }

func FromEnv(ctx context.Context, root, name string) (zero Context, err error) {
	log.Println("TROLOLOLO")
	envx.Debug(os.Environ()...)
	defer func() {
		log.Println("WAKA", spew.Sdump(zero))
	}()
	return Context{
		Module:      name,
		Root:        root,
		ModuleDir:   eg.ModuleDir,
		CacheDir:    eg.DefaultCacheDirectory(),
		RuntimeDir:  eg.DefaultRuntimeDirectory(),
		WorkingDir:  eg.DefaultWorkingDirectory(),
		WorkloadDir: eg.DefaultWorkloadDirectory(),
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

// NewLocal creates the workspace for a local run of eg using a unique root directory specific to that run.
func NewLocal(ctx context.Context, cid hash.Hash, cwd string, name string, options ...Option) (zero Context, err error) {
	workloadroot := filepath.Join(eg.WorkloadDirectory, fmt.Sprintf("%x", errorsx.Must(uuid.NewV7()).Bytes()[12:16]))
	return New(ctx, cid, filepath.Join(cwd, workloadroot), name, options...)
}

func New(ctx context.Context, cid hash.Hash, root string, name string, options ...Option) (zero Context, err error) {
	ignore := ignoredir{path: eg.CacheDirectory, reason: "cache directory"}

	if err = cacheid(ctx, root, eg.ModuleDir, cid, ignore); err != nil {
		return zero, errorsx.Wrap(err, "unable to create cache id")
	}

	_cid := uuid.FromBytesOrNil(cid.Sum(nil)).String()

	return ensuredirs(langx.Clone(Context{
		Module:      name,
		CachedID:    _cid,
		Root:        root,
		ModuleDir:   filepath.Join(root, eg.ModuleDir),
		CacheDir:    filepath.Join(root, eg.CacheDirectory),
		RuntimeDir:  filepath.Join(root, eg.RuntimeDirectory),
		WorkingDir:  filepath.Join(root, "mounted"),
		WorkloadDir: filepath.Join(root, eg.WorkloadDirectory),
		BuildDir:    filepath.Join(root, eg.CacheDirectory, eg.DefaultModuleDirectory(), ".gen", _cid, "build"),
		TransDir:    filepath.Join(root, eg.CacheDirectory, eg.DefaultModuleDirectory(), ".gen", _cid, "trans"),
		GenModDir:   filepath.Join(root, eg.CacheDirectory, eg.DefaultModuleDirectory(), ".gen", _cid, "trans", ".genmod"),
		Ignore:      ignore,
	}, options...))
}

func ensuredirs(c Context) (_ Context, err error) {
	rdir, err := os.Stat(c.Root)
	if err != nil {
		return c, err
	}

	perms := rdir.Mode() & (fs.ModePerm)

	debugx.Println("------------ ensuredirs initiated ------------")
	defer debugx.Println("------------ ensuredirs completed ------------")
	debugx.Println("perms", perms)
	debugx.Println(spew.Sdump(c))

	// need to ensure that certain directories and files exists
	// since they're mounted into the virtualized systems.
	return c, fsx.MkDirs(
		perms,
		filepath.Join(c.Root, c.GenModDir),
		filepath.Join(c.Root, c.BuildDir, c.Module, eg.ModuleDir),
		c.WorkloadDir,
		c.RuntimeDir,
	)
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
