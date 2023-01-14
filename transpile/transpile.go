package transpile

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/workspaces"
)

type Context struct {
	Workspace workspaces.Context
}

type Transpiler interface {
	Run(ctx context.Context, tctx Context) (roots []string, err error)
}

func New(ws workspaces.Context) Context {
	return Context{
		Workspace: ws,
	}
}

// Autodetect the transpiler to use.
func Autodetect() Transpiler {
	return golang{}
}

const Skip = errorsx.String("skipping content")

type golang struct{}

// TODO: need to have this actually rewrite all the source to another directory.
func (t golang) Run(ctx context.Context, tctx Context) (roots []string, err error) {
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
		return []string(nil), err
	}

	return roots, nil
}

func transpiledpath(tctx Context, current string) (path string, err error) {
	if path, err = filepath.Rel(tctx.Workspace.ModuleDir, current); err != nil {
		return "", err
	}
	return filepath.Join(tctx.Workspace.TransDir, path), nil
}

func visit(ctx context.Context, tctx Context, ignore func(string, fs.DirEntry) error, rewrite func(string, fs.DirEntry, []byte) error) error {
	return fs.WalkDir(os.DirFS(filepath.Join(tctx.Workspace.Root)), tctx.Workspace.ModuleDir, func(path string, d fs.DirEntry, err error) error {
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

		if cause := tctx.Workspace.Ignore.Ignore(path, d); cause != nil {
			log.Println("skipping", path, cause)
			if d.IsDir() {
				return fs.SkipDir
			}

			return nil
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

		if _, err = io.Copy(&buf, c); err != nil {
			return err
		}

		return rewrite(path, d, buf.Bytes())
	})
}
