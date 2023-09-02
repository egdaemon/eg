package ffigraph

import (
	"context"
	"fmt"
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

type traceevent func(ctx context.Context, m api.Module, idoffset uint32, idlen uint32) uint32

type grapher struct{}

func (t grapher) Pusher() traceevent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			id  string
			err error
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		log.Println("pushing", id)
		return 0
	}
}

func (t grapher) Popper() traceevent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			id  string
			err error
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		log.Println("popping", id)
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

func (t *graphviz) nid(s string) string {
	return fmt.Sprintf("n%s", s)
}

func (t *graphviz) Pusher() traceevent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			id  string
			err error
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		// id = t.nid(id)

		if id == t.current { // TODO find the cause.
			log.Println("DERP", id, t.current)
		}

		log.Println("pushing", id)
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

func (t *graphviz) Popper() traceevent {
	return func(ctx context.Context, m api.Module, idoffset, idlen uint32) uint32 {
		var (
			id  string
			err error
		)
		if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
			log.Println("unable to read id argument", err)
			return 1
		}

		// id = t.nid(id)

		log.Println("popping", id, len(t.stack), t.stack)
		t.stack = t.stack[:len(t.stack)-1]
		if len(t.stack) == 0 {
			return 0
		}

		t.current = t.stack[len(t.stack)-1]

		return 0
	}
}
