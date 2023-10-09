package ffigraph

import (
	"context"
	"log"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffi"
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

func New() noop {
	return noop{}
}

type TraceEvent func(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset uint32, idlen uint32) uint32

type Eventer interface {
	Pusher() TraceEvent
	Popper() TraceEvent
}
type noop struct{}

func (t noop) Pusher() TraceEvent {
	return func(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset, idlen uint32) uint32 {
		return 0
	}
}

func (t noop) Popper() TraceEvent {
	return func(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset, idlen uint32) uint32 {
		return 0
	}
}

func NewListener(g chan *EventInfo) *listener {
	return &listener{
		c: g,
	}
}

type State uint

const (
	Pushed State = iota
	Popped
)

type EventInfo struct {
	ID     string
	State  State
	Parent string
}

type listener struct {
	c chan *EventInfo
}

func (t *listener) Pusher() TraceEvent {
	return func(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset, idlen uint32) uint32 {
		var (
			pid string
			id  string
			err error
		)

		if pid, err = ffi.ReadString(m.Memory(), pidoffset, pidlen); err != nil {
			log.Println("unable to read pid argument", err)
			return 1
		}

		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		select {
		case <-ctx.Done():
			log.Println("unable to push event to listener", ctx.Err())
			return 1
		case t.c <- &EventInfo{ID: id, State: Pushed, Parent: pid}:
		}

		return 0
	}
}

func (t *listener) Popper() TraceEvent {
	return func(ctx context.Context, m api.Module, pidoffset uint32, pidlen uint32, idoffset, idlen uint32) uint32 {
		var (
			err error
			id  string
			pid string
		)

		if pid, err = ffi.ReadString(m.Memory(), pidoffset, pidlen); err != nil {
			log.Println("unable to read pid argument", err)
			return 1
		}

		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		select {
		case <-ctx.Done():
			log.Println("unable to push event to listener", ctx.Err())
			return 1
		case t.c <- &EventInfo{Parent: pid, ID: id, State: Popped}:
		}

		return 0
	}
}
