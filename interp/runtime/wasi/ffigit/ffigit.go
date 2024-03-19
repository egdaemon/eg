package ffigit

import (
	"context"
	"log"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
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
			treeish string
		)

		if treeish, err = ffi.ReadString(m.Memory(), treeishptr, treeishlen); err != nil {
			log.Println("unable to read treeish", err)
			return 1
		}

		digest, err := gitx.Commitish(dir, treeish)
		if err != nil {
			log.Println(errorsx.Wrap(err, "commit will be empty"))
			return 1
		}

		if !m.Memory().WriteString(commitptr, digest) {
			log.Println("failed to write hash length")
			return 1
		}

		if !m.Memory().WriteUint32Le(commitlen, uint32(len(digest))) {
			log.Println("failed to write hash length")
			return 1
		}

		return 0
	}
}
