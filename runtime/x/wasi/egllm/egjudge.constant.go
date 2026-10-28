package egllm

import (
	"context"
	"errors"
	"sync"
)

type constant struct {
	mu       sync.Mutex
	replies  []string
	requests [][]Message
}

// Constant is a Judge that answers each request with the next reply, useful in tests. a request beyond
// the replies is an error. the conversations it was given are available from Requests.
func Constant(replies ...string) *constant {
	return &constant{replies: replies}
}

func (t *constant) Do(_ context.Context, m ...Message) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.requests = append(t.requests, m)

	if len(t.requests) > len(t.replies) {
		return "", errors.New("unexpected request")
	}

	return t.replies[len(t.requests)-1], nil
}

// Requests are the conversations the judge was given, in order.
func (t *constant) Requests() [][]Message {
	t.mu.Lock()
	defer t.mu.Unlock()

	return append([][]Message(nil), t.requests...)
}
