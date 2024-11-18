// Package workspace is responsible for interacting with the workspace file system during the build.
package workspaces

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"hash"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
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
		RuntimeDir: "egruntime",
	}, nil
}

func New(ctx context.Context, root string, mdir string, name string) (zero Context, err error) {
	cidmd5 := md5.New()
	cdir := filepath.Join(mdir, ".cache")
	runtimedir := filepath.Join(mdir, ".egruntime")
	ignore := ignoredir{path: cdir, reason: "cache directory"}

	if err = cacheid(ctx, root, mdir, cidmd5, ignore); err != nil {
		return zero, errorsx.Wrap(err, "unable to create cache id")
	}

	cid := hex.EncodeToString(cidmd5.Sum(nil))

	return ensuredirs(Context{
		Module:     name,
		CachedID:   cid,
		Root:       root,
		ModuleDir:  mdir,
		CacheDir:   cdir,
		RuntimeDir: runtimedir,
		WorkingDir: filepath.Join(runtimedir, "mounted"),
		BuildDir:   filepath.Join(cdir, "build", cid),
		TransDir:   filepath.Join(cdir, "trans", cid),
		GenModDir:  filepath.Join(cdir, "trans", cid, ".genmod"),
		Ignore:     ignore,
	})
}

func ensuredirs(c Context) (_ Context, err error) {
	// need to ensure that certain directories and files exists
	// since they're mounted into containers.
	return c, fsx.MkDirs(
		0700,
		filepath.Join(c.Root, c.RuntimeDir),
		filepath.Join(c.Root, c.WorkingDir),
		filepath.Join(c.Root, c.GenModDir),
		filepath.Join(c.Root, c.BuildDir, c.Module, "main.wasm.d"),
		filepath.Join(c.Root, c.CacheDir),
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
