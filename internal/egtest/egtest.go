package egtest

import (
	"context"
	"sync"

	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

type op string

func (t op) ID() string {
	return string(t)
}

func Op() eg.Op {
	return op(uuidx.NewV4().String())
}

func NewBuffer() *buffer {
	return &buffer{}
}

type buffer struct {
	sync.Mutex
	b []byte
}

func (t *buffer) Op(b ...byte) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		t.Mutex.Lock()
		defer t.Mutex.Unlock()
		t.b = append(t.b, b...)
		return nil
	}
}

func (t *buffer) Current() []byte {
	return t.b
}
