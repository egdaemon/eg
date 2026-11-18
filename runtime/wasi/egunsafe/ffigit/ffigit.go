package ffigit

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

func Bearer() string {
	envtoken := envx.String("", eg.EnvGitAuthHTTPPassword, eg.EnvGithubToken)
	if stringsx.Present(envtoken) {
		return envtoken
	}

	var buf = make([]byte, 1024)
	tokenptr, tokenlen := ffiguest.Bytes(buf)
	errcode := bearer(tokenptr, tokenlen)
	if err := ffiguest.Error(errcode, fmt.Errorf("bearer failed")); err != nil {
		log.Println("unable to get bearer token", err)
		return ""
	}
	return string(bytes.TrimRight(ffiguest.BytesRead(tokenptr, tokenlen), "\x00"))
}

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

type cloneopts struct {
	environ []string
}

// Option modifies how Clone calls the host.
type Option func(*cloneopts)

// Environ replaces the environment sent to the host with env. the host reads its clone behavior from it,
// e.g.) EG_UNSAFE_GIT_CLONE_ENABLED, as well as any credentials. the default is os.Environ().
func Environ(env ...string) Option {
	return func(o *cloneopts) {
		o.environ = env
	}
}

func newcloneopts(opts ...Option) (o cloneopts) {
	o.environ = os.Environ()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func Clone(ctx context.Context, dir, uri, remote, branch string, opts ...Option) error {
	o := newcloneopts(opts...)
	dirptr, dirlen := ffiguest.String(dir)
	uriptr, urilen := ffiguest.String(uri)
	remoteptr, remotelen := ffiguest.String(remote)
	treeishptr, treeishlen := ffiguest.String(branch)
	envptr, envsize, envlen := ffiguest.StringArray(o.environ...)

	errcode := clone3(
		ffiguest.ContextDeadline(ctx),
		dirptr, dirlen,
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
