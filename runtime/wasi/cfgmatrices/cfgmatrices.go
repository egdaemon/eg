package cfgmatrices

import (
	"context"

	"github.com/james-lawrence/eg/runtime/wasi/yak"
)

type Builder[T any] interface {
	Boolean(func(dst *T, v bool)) Builder[T]
	String(m func(dst *T, v string), options ...string) Builder[T]
	Int64(m func(dst *T, v int64), options ...string) Builder[T]
	Float64(m func(dst *T, v float64), options ...string) Builder[T]
	Perform(ctx context.Context, tasks ...yak.OpFn) error
}

func New[T any]() Builder[T] {
	return nil
}