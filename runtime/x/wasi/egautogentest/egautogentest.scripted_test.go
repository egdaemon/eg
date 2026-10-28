package egautogentest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

// scripted is an egllm.Judge that answers each request with the next reply and records the messages sent.
type scripted struct {
	replies  []string
	requests [][]egllm.Message
}

func (t *scripted) Do(_ context.Context, m ...egllm.Message) (string, error) {
	t.requests = append(t.requests, m)

	if len(t.requests) > len(t.replies) {
		return "", errors.New("unexpected request")
	}

	return t.replies[len(t.requests)-1], nil
}

func script(_ *testing.T, replies ...string) *scripted {
	return &scripted{replies: replies}
}

func violationrecord(rule int, line int, reason string) string {
	return fmt.Sprintf(`{"rule": %d, "line": %d, "reason": %q}`, rule, line, reason)
}
