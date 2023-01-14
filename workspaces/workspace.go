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

	"github.com/james-lawrence/eg/internal/errorsx"
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
	CachedID  string // unique id generated from the content of the module.
	Root      string // workspace root directory.
	ModuleDir string // eg module directory; relative to the root
	CacheDir  string // cache directory. relative to the module directory.
	BuildDir  string // directory for built wasm modules; relative to the cache directory.
	TransDir  string // root directory for the transpiled code; relative to the cache directory.
	Ignore    ignorable
}

func (t Context) FS() fs.FS {
	return os.DirFS(t.Root)
}

func New(ctx context.Context, root string, mdir string) (zero Context, err error) {
	cidmd5 := md5.New()
	cdir := filepath.Join(mdir, ".cache")
	ignore := ignoredir{path: cdir, reason: "cache directory"}

	if err = cacheid(ctx, root, mdir, cidmd5, ignore); err != nil {
		return zero, err
	}

	cid := hex.EncodeToString(cidmd5.Sum(nil))

	return Context{
		CachedID:  cid,
		Root:      root,
		ModuleDir: mdir,
		CacheDir:  cdir,
		BuildDir:  filepath.Join(cdir, ".eg", "build", cid),
		TransDir:  filepath.Join(cdir, ".eg", "trans", cid),
		Ignore:    ignore,
	}, nil
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
