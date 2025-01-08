package ffigraph

import (
	"context"
	"log"

	"github.com/tetratelabs/wazero/api"
)

func Analysing(b bool) func(ctx context.Context, m api.Module) uint32 {
	return func(
		ctx context.Context,
		m api.Module,
	) uint32 {
		if b {
			return 0
		}

		return 1
	}
}

func NoopTraceEventPush(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset, idlen uint32) uint32 {
	log.Println("removed push method invoked")
	return 0
}

func NoopTraceEventPop(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset, idlen uint32) uint32 {
	log.Println("removed pop method invoked")
	return 0
}
