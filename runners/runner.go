package runners

import (
	"context"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
)

// T runs compiled modules, one at a time, in a runtime: containers, a machine or the host.
type T interface {
	Do(ctx context.Context, m transpile.Compiled) error
	Close() error
}

// B prepares a runtime for the modules of the workspace, adding what the runtime needs to their
// environment.
type B interface {
	New(ctx context.Context, ws workspaces.Context, uid uuid.UUID, canonicaluri string, envb *envx.Builder) (T, error)
}

// BFn is a function that implements B.
type BFn func(ctx context.Context, ws workspaces.Context, uid uuid.UUID, canonicaluri string, envb *envx.Builder) (T, error)

func (t BFn) New(ctx context.Context, ws workspaces.Context, uid uuid.UUID, canonicaluri string, envb *envx.Builder) (T, error) {
	return t(ctx, ws, uid, canonicaluri, envb)
}
