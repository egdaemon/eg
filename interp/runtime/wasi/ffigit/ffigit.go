package ffigit

import (
	"context"
	"log"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/tetratelabs/wazero/api"
)

func Commitish(dir string) func(
	ctx context.Context,
	m api.Module,
	deadline int64, // context.Context
	treeishptr uint32, treeishlen uint32, // string
	commitptr uint32, commitlen uint32, // return string
) (errcode uint32) {
	return func(ctx context.Context, m api.Module, deadline int64, treeishptr, treeishlen, commitptr, commitlen uint32) (errcode uint32) {
		_, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		var (
			err     error
			r       *git.Repository
			hash    *plumbing.Hash
			treeish string
		)

		if treeish, err = ffi.ReadString(m.Memory(), treeishptr, treeishlen); err != nil {
			log.Println("unable to read treeish", err)
			return 1
		}

		if r, err = git.PlainOpen(dir); err != nil {
			log.Println("unable to detect git repository - commit will be empty", dir, err)
			return 1
		}

		if hash, err = r.ResolveRevision(plumbing.Revision(treeish)); err != nil {
			log.Println("unable to resolve git reference - commit will be empty", dir, treeish, err)
			return 1
		}

		s := hash.String()

		if !m.Memory().WriteString(commitptr, s) {
			return 1
		}

		if !m.Memory().WriteUint32Le(commitlen, uint32(len(s))) {
			log.Println("failed to write string length")
			return 1
		}

		return 0
	}
}
