package fficoreutils

import (
	"context"
	"fmt"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExitError is returned when the utility exits with a non zero code.
type ExitError struct {
	Name string
	Code int
}

func (t ExitError) Error() string {
	return fmt.Sprintf("%s exited with code %d", t.Name, t.Code)
}

// Run executes the coreutils utility with the given arguments (argv[0] is supplied by the host).
// the utility uses this module's stdio, which the host wires up when it runs the module.
func Run(ctx context.Context, name string, args ...string) error {
	nameptr, namelen := ffiguest.String(name)
	argsptr, argslen, argssize := ffiguest.StringArray(args...)

	code := run(ffiguest.ContextDeadline(ctx), nameptr, namelen, argsptr, argslen, argssize)
	if code == 0 {
		return nil
	}

	return ExitError{Name: name, Code: int(code)}
}

// Proxy is an exec proxy client for the runtime's coreutils, so the request's stdio is the guest's sockets in
// proc/{id} and execproxy/v2.Run drives it like the module. req.Dir is a path within the guest's mounts. a
// utility the runtime has no coreutils module, or no export, for fails with codes.NotFound, as does every
// utility outside of a wasm guest.
type Proxy struct{}

func (Proxy) Exec(ctx context.Context, req *execproxy.ExecRequest, _ ...grpc.CallOption) (*execproxy.ExecResponse, error) {
	var (
		idptr, idlen               = ffiguest.String(req.Id)
		dirptr, dirlen             = ffiguest.String(req.Dir)
		nameptr, namelen           = ffiguest.String(req.Cmd)
		argsptr, argslen, argssize = ffiguest.StringArray(req.Arguments...)
	)

	switch code := start(ffiguest.ContextDeadline(ctx), idptr, idlen, dirptr, dirlen, nameptr, namelen, argsptr, argslen, argssize); code {
	case 0:
	case ffierrors.ErrNotFound, ffierrors.ErrNotImplemented:
		return nil, status.Errorf(codes.NotFound, "coreutils does not provide %s", req.Cmd)
	default:
		return nil, ffiguest.Error(code, errorsx.Errorf("unable to start %s", req.Cmd))
	}

	// the host can not block until the utility exits: the guest is a single thread, and has to copy the
	// utility's stdio meanwhile.
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
