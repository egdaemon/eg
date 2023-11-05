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

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/pkg/errors"
)

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
	Module    string // name of the module
	CachedID  string // unique id generated from the content of the module.
	Root      string // workspace root directory.
	ModuleDir string // eg module directory; relative to the root
	CacheDir  string // cache directory. relative to the module directory.
	RunnerDir string // cache directory for the runner.
	BuildDir  string // directory for built wasm modules; relative to the cache directory.
	TransDir  string // root directory for the transpiled code; relative to the cache directory.
	GenModDir string // root directory for generated modules; relative to the cache directory.
	Ignore    ignorable
}

func (t Context) FS() fs.FS {
	return os.DirFS(t.Root)
}

func New(ctx context.Context, root string, mdir string, name string) (zero Context, err error) {
	cidmd5 := md5.New()
	cdir := filepath.Join(mdir, ".cache")
	ignore := ignoredir{path: cdir, reason: "cache directory"}

	if err = cacheid(ctx, root, mdir, cidmd5, ignore); err != nil {
		return zero, err
	}

	cid := hex.EncodeToString(cidmd5.Sum(nil))

	return ensuredirs(Context{
		Module:    name,
		CachedID:  cid,
		Root:      root,
		ModuleDir: mdir,
		CacheDir:  cdir,
		RunnerDir: filepath.Join(cdir, ".eg"),
		BuildDir:  filepath.Join(cdir, ".eg", "build", cid),
		TransDir:  filepath.Join(cdir, ".eg", "trans", cid),
		GenModDir: filepath.Join(cdir, ".eg", "trans", cid, ".genmod"),
		Ignore:    ignore,
	})
}

func ensuredirs(c Context) (_ Context, err error) {
	mkdirs := func(paths ...string) error {
		for _, p := range paths {
			if err = os.MkdirAll(p, 0700); err != nil {
				return errors.Wrapf(err, "unable to create directory: %s", p)
			}
		}

		return nil
	}

	return c, mkdirs(c.GenModDir, c.BuildDir)
}

func cacheid(ctx context.Context, root string, mdir string, cacheid hash.Hash, ignore ignorable) error {
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

		if c, err = os.Open(path); err != nil {
			return err
		}

		if _, err = io.Copy(cacheid, c); err != nil {
			return err
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
