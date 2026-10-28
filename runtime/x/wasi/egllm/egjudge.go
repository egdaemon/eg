package egllm

import (
	"context"
	"fmt"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

// Judge generates a response to a conversation.
type Judge interface {
	Do(ctx context.Context, m ...Message) (string, error)
}

// Server is implemented by judges that need setup and teardown around the operations
// that use them, e.g.) a managed llama-server.
type Server interface {
	// With runs ops in sequence while the judge is available.
	With(ops ...eg.OpFn) eg.OpFn
}

type llamacpp struct {
	model    string
	endpoint string // empty defers to Endpoint().
}

// LLamaCPP is a Judge backed by a llama.cpp server running model, see With.
func LLamaCPP(model string) llamacpp {
	return llamacpp{model: model}
}

// Endpoint sets the url of the server, an http(s) url or a unix socket, see unixScheme.
// the server is then external and not managed by With. defaults to the package Endpoint().
func (t llamacpp) Endpoint(v string) llamacpp {
	t.endpoint = strings.TrimRight(v, "/")
	return t
}

func (t llamacpp) Do(ctx context.Context, m ...Message) (string, error) {
	if t.endpoint == "" {
		return Chat(ctx, New(), t.model, m...)
	}

	return Chat(ctx, newClient(t.endpoint), t.model, m...)
}

func (t llamacpp) With(ops ...eg.OpFn) eg.OpFn {
	op := eg.Sequential(ops...)
	if len(ops) == 1 {
		op = ops[0]
	}

	if t.endpoint == "" {
		return With(t.model, op)
	}

	return external(t.endpoint, op)
}

type echo struct{}

// Echo is a Judge that responds with the conversation it was given, useful when no model is available.
func Echo() Judge {
	return echo{}
}

func (echo) Do(_ context.Context, m ...Message) (string, error) {
	var b strings.Builder
	for i, msg := range m {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "-------------- %s initiated --------------\n%s\n-------------- %s completed --------------", msg.Role, msg.Content, msg.Role)
	}

	return b.String(), nil
}
