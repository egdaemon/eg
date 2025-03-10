package ffigit

import (
	"context"
	"log"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
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

func CloneV1(dir string) func(
	ctx context.Context,
	m api.Module,
	deadline int64, // context.Context
	uriptr uint32, urilen uint32, // string
	remoteptr uint32, remotelen uint32, // string
	treeishptr uint32, treeishlen uint32, // string
) (errcode uint32) {
	return func(gctx context.Context, m api.Module, deadline int64, uriptr, urilen, remoteptr, remotelen, treeishptr, treeishlen uint32) (errcode uint32) {
		ctx, done := ffi.ReadMicroDeadline(gctx, deadline)
		defer done()

		var (
			err     error
			uri     string
			remote  string
			treeish string
			auth    transport.AuthMethod
		)

		if uri, err = ffi.ReadString(m.Memory(), uriptr, urilen); err != nil {
			log.Println("unable to read uri", err)
			return 1
		}

		if remote, err = ffi.ReadString(m.Memory(), remoteptr, remotelen); err != nil {
			log.Println("unable to read remote", err)
			return 1
		}

		if treeish, err = ffi.ReadString(m.Memory(), treeishptr, treeishlen); err != nil {
			log.Println("unable to read treeish", err)
			return 1
		}

		if username, password := envx.String("", gitx.EnvAuthHTTPUsername), envx.String("", gitx.EnvAuthHTTPPassword); !(stringsx.Blank(username) || !stringsx.Blank(password)) {
			auth = &githttp.BasicAuth{Username: username, Password: password}
		}

		if err := gitx.Clone(ctx, auth, dir, uri, remote, treeish); err != nil {
			log.Println(errorsx.Wrap(err, "clone failed"))
			return 1
		}

		return 0
	}
}

func CloneV2(dir string, runtimedir string) func(
	ctx context.Context,
	m api.Module,
	deadline int64, // context.Context
	uriptr uint32, urilen uint32, // string
	remoteptr uint32, remotelen uint32, // string
	treeishptr uint32, treeishlen uint32, // string
	envoffset uint32, envlen uint32, envsize uint32, // []string
) (errcode uint32) {
	return func(
		gctx context.Context,
		m api.Module,
		deadline int64,
		uriptr, urilen uint32,
		remoteptr, remotelen uint32,
		treeishptr, treeishlen uint32,
		envoffset uint32, envlen uint32, envsize uint32, // []string
	) (errcode uint32) {
		ctx, done := ffi.ReadMicroDeadline(gctx, deadline)
		defer done()

		var (
			err     error
			uri     string
			remote  string
			treeish string
			env     []string
		)

		if uri, err = ffi.ReadString(m.Memory(), uriptr, urilen); err != nil {
			log.Println("unable to read uri", err)
			return 1
		}

		if remote, err = ffi.ReadString(m.Memory(), remoteptr, remotelen); err != nil {
			log.Println("unable to read remote", err)
			return 1
		}

		if treeish, err = ffi.ReadString(m.Memory(), treeishptr, treeishlen); err != nil {
			log.Println("unable to read treeish", err)
			return 1
		}

		if env, err = ffi.ReadStringArray(m.Memory(), envoffset, envlen, envsize); err != nil {
			log.Println("unable to read environment variables", err)
			return 1
		}

		environ := envx.NewEnvironFromStrings(env...)
		// envx.Debug(env...)
		autoauth := func() transport.AuthMethod {
			if auth, err := gitx.LoadCredentials(ctx, uri, runtimedir); auth != nil {
				log.Println("git access token detected", auth)
				return auth
			} else if err != nil {
				log.Println("unable to load git credentials", err)
			}

			if username, password := environ.String("", gitx.EnvAuthHTTPUsername), environ.String("", gitx.EnvAuthHTTPPassword); !stringsx.Blank(username) && !stringsx.Blank(password) {
				log.Println("git http auth detected")
				return &githttp.BasicAuth{Username: username, Password: password}
			}

			log.Println("no auth detected")
			return nil
		}

		// this is a hack to disable cloning in local environment. we need to work on improving
		// go-git to support this case cleanly.
		if environ.Boolean(true, eg.EnvUnsafeGitCloneEnabled) {
			if err := gitx.Clone(ctx, autoauth(), dir, uri, remote, treeish); err != nil {
				log.Println(errorsx.Wrap(err, "clone failed"))
				return 1
			}
		} else {
			log.Printf("skipping clone due to %s flag being false", eg.EnvUnsafeGitCloneEnabled)
		}

		return 0
	}
}
