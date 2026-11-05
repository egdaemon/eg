// Package ffiwasi runs commands from the runtime's set of WASI modules, inside the runtime the guest runs in.
package ffiwasi

import (
	"context"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Proxy is an exec proxy client for the runtime's module set, so the request's stdio is the guest's sockets in
// proc/{id}. a command the set has no module for fails with codes.NotFound, as does every command outside of a
// wasm guest.
type Proxy struct{}

func (Proxy) Exec(ctx context.Context, req *execproxy.ExecRequest, _ ...grpc.CallOption) (*execproxy.ExecResponse, error) {
	var (
		idptr, idlen               = ffiguest.String(req.Id)
		dirptr, dirlen             = ffiguest.String(req.Dir)
		envptr, envlen, envsize    = ffiguest.StringArray(req.Environment...)
		nameptr, namelen           = ffiguest.String(req.Cmd)
		argsptr, argslen, argssize = ffiguest.StringArray(req.Arguments...)
	)

	switch code := start(idptr, idlen, dirptr, dirlen, envptr, envlen, envsize, nameptr, namelen, argsptr, argslen, argssize); code {
	case 0:
	case ffierrors.ErrNotFound, ffierrors.ErrNotImplemented:
		return nil, status.Errorf(codes.NotFound, "no wasi module for %s", req.Cmd)
	default:
		return nil, ffiguest.Error(code, errorsx.Errorf("unable to start %s", req.Cmd))
	}

	// the host can not block until the command exits: the guest is a single thread, and it has to copy the
	// command's stdio meanwhile.
	for {
		switch code := wait(idptr, idlen); code {
		case ffierrors.ErrPending:
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Millisecond):
			}
		case ffierrors.ErrNotFound, ffierrors.ErrUnrecoverable:
			return nil, ffiguest.Error(code, errorsx.Errorf("unable to wait on %s", req.Cmd))
		default:
			return &execproxy.ExecResponse{Exit: int32(code)}, nil
		}
	}
}
