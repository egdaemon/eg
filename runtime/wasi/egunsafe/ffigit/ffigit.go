package ffigit

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
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

func Clone(ctx context.Context, uri, remote, branch string) error {
	uriptr, urilen := ffiguest.String(uri)
	remoteptr, remotelen := ffiguest.String(remote)
	treeishptr, treeishlen := ffiguest.String(branch)
	envptr, envsize, envlen := ffiguest.StringArray(os.Environ()...)

	errcode := clone2(
		ffiguest.ContextDeadline(ctx),
		uriptr, urilen,
		remoteptr, remotelen,
		treeishptr, treeishlen,
		envptr, envsize, envlen,
	)

	if err := ffiguest.Error(errcode, fmt.Errorf("clone failed")); err != nil {
		return err
	}

	return nil
}
