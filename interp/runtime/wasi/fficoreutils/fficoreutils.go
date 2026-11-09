// Package fficoreutils stands in for the egcoreutils reactor when the runtime has none.
package fficoreutils

import (
	"context"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Name is the module guests import the reactor's functions from, the name wasi.mod.d/egcoreutils.wasi
// loads under.
const Name = "egcoreutils"

// Unavailable instantiates an egcoreutils module that provides no utilities, unless the runtime already loaded
// the reactor. guests import its functions directly, so without it they could not start at all; with it every
// command they give the reactor falls through to the module.
func Unavailable(ctx context.Context, rt wazero.Runtime) (api.Closer, error) {
	if rt.Module(Name) != nil {
		return loaded{}, nil
	}

	unavailable := func(ctx context.Context, id uint64) uint32 { return ffierrors.ErrNotImplemented }
	return rt.NewHostModuleBuilder(Name).
		NewFunctionBuilder().WithFunc(unavailable).Export("execveatinit").
		NewFunctionBuilder().WithFunc(unavailable).Export("execveatwait").
		Instantiate(ctx)
}

// loaded leaves the reactor to whoever loaded it.
type loaded struct{}

func (loaded) Close(context.Context) error { return nil }
