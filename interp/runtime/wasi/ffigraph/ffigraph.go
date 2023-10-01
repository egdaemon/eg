package ffigraph

import (
	"context"
	"log"

	"github.com/awalterschulze/gographviz"
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

func New() grapher {
	return grapher{}
}

type TraceEvent func(ctx context.Context, m api.Module, idoffset uint32, idlen uint32) uint32

type Eventer interface {
	Pusher() TraceEvent
	Popper() TraceEvent
}
type grapher struct{}

func (t grapher) Pusher() TraceEvent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		// var (
		// 	id  string
		// 	err error
		// )
		// if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
		// 	log.Println("unable to read id argument", err)
		// 	return 1
		// }

		// log.Println("pushing", id)
		return 0
	}
}

func (t grapher) Popper() TraceEvent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		// var (
		// 	id  string
		// 	err error
		// )
		// if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
		// 	log.Println("unable to read id argument", err)
		// 	return 1
		// }

		// log.Println("popping", id)
		return 0
	}
}

func NewListener(g chan *EventInfo) *listener {
	return &listener{
		c:       g,
		stack:   []string{},
		current: "",
	}
}

type State uint

const (
	Pushed State = iota
	Popped
)

type EventInfo struct {
	ID    string
	State State
}

type listener struct {
	c       chan *EventInfo
	stack   []string
	current string
}

func (t *listener) Pusher() TraceEvent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			id  string
			err error
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		if t.current != "" {
			select {
			case <-ctx.Done():
				log.Println("unable to push event to listener", ctx.Err())
				return 1
			case t.c <- &EventInfo{ID: id, State: Pushed}:
			}
		}

		t.current = id
		t.stack = append(t.stack, id)

		return 0
	}
}

func (t *listener) Popper() TraceEvent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			err error
			id  string
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		select {
		case <-ctx.Done():
			log.Println("unable to push event to listener", ctx.Err())
			return 1
		case t.c <- &EventInfo{ID: id, State: Popped}:
		}

		return 0
	}
}

func NewViz(g *gographviz.Graph) *graphviz {
	return &graphviz{
		g:       g,
		stack:   []string{},
		current: "",
	}
}

type graphviz struct {
	g       *gographviz.Graph
	stack   []string
	current string
}

func (t *graphviz) Pusher() TraceEvent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			id  string
			err error
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		if err = t.g.AddNode(t.current, id, nil); err != nil {
			log.Println("unable to trace graph node", err)
			return 1
		}

		if t.current != "" {
			if err = t.g.AddEdge(t.current, id, true, nil); err != nil {
				log.Println("unable to trace graph edge", err)
				return 1
			}
		}

		t.current = id
		t.stack = append(t.stack, id)

		return 0
	}
}

func (t *graphviz) Popper() TraceEvent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			err error
		)
		if _, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		t.stack = t.stack[:len(t.stack)-1]
		if len(t.stack) == 0 {
			return 0
		}

		t.current = t.stack[len(t.stack)-1]

		return 0
	}
}
