package egtest

import (
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/gofrs/uuid"
)

type op string

func (t op) ID() string {
	return string(t)
}

func Op() eg.Op {
	return op(errorsx.Zero(uuid.NewV4()).String())
}
