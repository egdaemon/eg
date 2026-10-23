package workspaces

import (
	"context"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
)

func PrepareBuiltin(ctx context.Context, root string, archive fs.FS) error {
	if err := fsx.MkDirs(0700, root); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}

	if err := fsx.CloneTree(ctx, root, ".builtin", archive); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	return nil
}

// BuiltinWorkloads scans the embedded archive for builtin workloads: directories
// under .builtin containing a non-test go file. the .builtin root itself is not
// a workload. paths keep the .builtin prefix (e.g. .builtin/cache/clear), which
// Workloads can never produce (go's ./... skips dot-directories), so consumers
// can tell builtin from repo workloads and strip it to get the builtin name.
func BuiltinWorkloads(ctx context.Context, archive fs.FS) ([]Desc, error) {
	var found []Desc

	err := fs.WalkDir(archive, ".builtin", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if path.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") {
			return nil
		}

		dir := path.Dir(p)
		if dir == ".builtin" {
			return nil
		}

		if !slices.ContainsFunc(found, func(d Desc) bool { return d.Path == dir }) {
			found = append(found, Desc{Path: dir})
		}

		return nil
	})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to scan builtin workloads")
	}

	return found, nil
}
