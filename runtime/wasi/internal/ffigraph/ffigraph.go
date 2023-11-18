package ffigraph

import (
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
)

func Analysing() bool {
	return analysing() == 0
}

type node interface {
	ID() string
}

func WrapErr(parent, op node, fn func() error) error {
	pid, plen := ffiguest.String("")
	if parent != nil {
		pid, plen = ffiguest.String(parent.ID())
	}

	oid, olen := ffiguest.String(op.ID())
	push(pid, plen, oid, olen)
	defer pop(pid, plen, oid, olen)

	return fn()
}
