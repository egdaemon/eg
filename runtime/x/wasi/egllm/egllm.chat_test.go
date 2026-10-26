package egllm_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

type client struct {
	srv *httptest.Server
}

func (t client) Do(r *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(r)
}

func (t client) Host() string {
	return t.srv.URL
}

func serve(t *testing.T, sent *[]egllm.Message) client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req struct {
			Messages []egllm.Message `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(raw, &req))
		*sent = req.Messages

		_, err = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"reply"}}]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	return client{srv: srv}
}

func TestChat(t *testing.T) {
	t.Run("sends every message in order", func(t *testing.T) {
		var sent []egllm.Message
		c := serve(t, &sent)

		got, err := egllm.Chat(t.Context(), c, "m",
			egllm.Message{Role: "user", Content: "a"},
			egllm.Message{Role: "assistant", Content: "b"},
			egllm.Message{Role: "user", Content: "c"},
		)
		require.NoError(t, err)
		require.Equal(t, "reply", got)
		require.Equal(t, []egllm.Message{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}, {Role: "user", Content: "c"}}, sent)
	})
}

func TestGenerate(t *testing.T) {
	t.Run("sends the prompt as a single user message", func(t *testing.T) {
		var sent []egllm.Message
		c := serve(t, &sent)

		got, err := egllm.Generate(t.Context(), c, "m", "prompt")
		require.NoError(t, err)
		require.Equal(t, "reply", got)
		require.Equal(t, []egllm.Message{{Role: "user", Content: "prompt"}}, sent)
	})
}
