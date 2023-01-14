package transpile

import (
	"bytes"
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
)

type Context struct {
	CacheID   hash.Hash
	Workspace fs.FS
	root      string
	cachedir  string // the directory we rewrite content into and is ignored for checksumming purposes
}

type Transpiler interface {
	Run(ctx context.Context, tctx Context) (cacheid string, roots []string, err error)
}

func New(workspace fs.FS, root string, cachedir string) Context {
	return Context{
		CacheID:   md5.New(),
		Workspace: workspace,
		root:      root,
		cachedir:  cachedir,
	}
}

// Autodetect the transpiler to use.
func Autodetect() Transpiler {
	return golang{}
}

const Skip = errorsx.String("skipping content")

type golang struct{}

// TODO: need to have this actually rewrite all the source to another directory.
func (t golang) Run(ctx context.Context, tctx Context) (_ string, roots []string, err error) {
	ignore := func(path string, d fs.DirEntry) error {
		if !strings.HasSuffix(path, ".go") {
			return errorsx.String("ignoring non-golang file")
		}

		return nil
	}

	rewrite := func(from string, d fs.DirEntry, contents []byte) (err error) {
		var (
			dst string
		)

		if dst, err = transpiledpath(tctx, from); err != nil {
			return err
		}

		if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}

		if err = os.WriteFile(dst, contents, 0600); err != nil {
			return err
		}

		if bytes.Contains(contents, []byte("func main()")) {
			roots = append(roots, dst)
		}

		return nil
	}

	if err = visit(ctx, tctx, ignore, rewrite); err != nil {
		return "", []string(nil), err
	}

	return hex.EncodeToString(tctx.CacheID.Sum(nil)), roots, nil
}

func transpiledpath(tctx Context, current string) (path string, err error) {
	if path, err = filepath.Rel(tctx.root, current); err != nil {
		return "", err
	}

	return filepath.Join(tctx.root, tctx.cachedir, "transpile", path), nil
}

func visit(ctx context.Context, tctx Context, ignore func(string, fs.DirEntry) error, rewrite func(string, fs.DirEntry, []byte) error) error {
	return fs.WalkDir(tctx.Workspace, tctx.root, func(path string, d fs.DirEntry, err error) error {
		var (
			c   *os.File
			buf bytes.Buffer
		)

		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// in case it isn't already in the ignorable set
		if strings.HasSuffix(tctx.cachedir, d.Name()) {
			log.Println("skipping cache directory", path)
			return fs.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if cause := ignore(path, d); cause != nil {
			log.Println("skipping", path, cause)
			return nil
		}

		if c, err = os.Open(path); err != nil {
			return err
		}

		if _, err = io.Copy(io.MultiWriter(tctx.CacheID, &buf), c); err != nil {
			return err
		}

		return rewrite(path, d, buf.Bytes())
	})
}
