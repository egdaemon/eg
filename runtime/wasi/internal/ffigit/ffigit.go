package ffigit

import (
	"context"
	"fmt"
	"log"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
)

func Commitish(ctx context.Context, treeish string) string {
	var (
		buf = make([]byte, 40)
	)
	treeishptr, treeishlen := ffiguest.String(treeish)
	revisionptr, revisionlen := ffiguest.Bytes(buf)

	errcode := commitish(
		ffiguest.ContextDeadline(ctx),
		treeishptr, treeishlen,
		revisionptr, revisionlen,
	)
	if err := ffiguest.Error(errcode, fmt.Errorf("commitish failed")); err != nil {
		log.Println("unable to detect commit", err)
		return ""
	}

	return string(ffiguest.BytesRead(revisionptr, revisionlen))
}
