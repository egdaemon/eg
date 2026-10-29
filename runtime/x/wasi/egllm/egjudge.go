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
	budget   int    // reasoning budget in tokens, zero defers to the server.
	effort   string // reasoning effort for the chat template, empty defers to the template.
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

// ReasoningBudget caps the tokens the model reasons for before answering each request, e.g.) 2048 for a moderate effort.
// a --reasoning-budget on the server's command line overrides it. defaults to the server's.
func (t llamacpp) ReasoningBudget(tokens int) llamacpp {
	t.budget = tokens
	return t
}

// ReasoningEffort sets the chat template's reasoning_effort for each request, e.g.) "medium".
// the template decides which values are valid, an unsupported one fails the request.
// defaults to the template's.
func (t llamacpp) ReasoningEffort(effort string) llamacpp {
	t.effort = effort
	return t
}

func (t llamacpp) Do(ctx context.Context, m ...Message) (string, error) {
	c := New()
	if t.endpoint != "" {
		c = newClient(t.endpoint)
	}

	r := request{Model: t.model, Messages: m, ReasoningBudgetTokens: t.budget}
	if t.effort != "" {
		r.ChatTemplateKwargs = map[string]any{"reasoning_effort": t.effort}
	}

	return chat(ctx, c, r)
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
