package egautogentest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

// scripted is an egllm.Client that answers each request with the next reply and records the messages sent.
type scripted struct {
	srv      *httptest.Server
	requests [][]egllm.Message
}

func (t *scripted) Do(r *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(r)
}

func (t *scripted) Host() string {
	return t.srv.URL
}

func script(t *testing.T, replies ...string) *scripted {
	s := &scripted{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req struct {
			Messages []egllm.Message `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(raw, &req))
		s.requests = append(s.requests, req.Messages)

		if len(s.requests) > len(replies) {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}

		encoded, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": replies[len(s.requests)-1]}}}})
		require.NoError(t, err)
		_, err = w.Write(encoded)
		require.NoError(t, err)
	}))
	t.Cleanup(s.srv.Close)

	return s
}

func violationrecord(rule int, line int, reason string) string {
	return fmt.Sprintf(`{"rule": %d, "line": %d, "reason": %q}`, rule, line, reason)
}
