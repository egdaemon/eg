package egllm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/egdaemon/eg"
	wasieg "github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestLLamaCPPDo(t *testing.T) {
	type sent struct {
		Path     string
		Model    string          `json:"model"`
		Messages []egllm.Message `json:"messages"`
	}

	serveEndpoint := func(t *testing.T, status int, body string, got *sent) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, got))
			got.Path = r.URL.Path

			w.WriteHeader(status)
			_, err = w.Write([]byte(body))
			require.NoError(t, err)
		}))
		t.Cleanup(srv.Close)
		t.Setenv(eg.EnvComputeLLMEndpoint, srv.URL)
	}

	t.Run("posts the model and every message in order to the chat completions endpoint", func(t *testing.T) {
		var got sent
		serveEndpoint(t, http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"reply"}}]}`, &got)

		messages := []egllm.Message{
			{Role: "user", Content: "a"},
			{Role: "assistant", Content: "b"},
			{Role: "user", Content: "c"},
		}
		reply, err := egllm.LLamaCPP("org/model:Q4").Do(t.Context(), messages...)
		require.NoError(t, err)
		require.Equal(t, "reply", reply)
		require.Equal(t, "/v1/chat/completions", got.Path)
		require.Equal(t, "org/model:Q4", got.Model)
		require.Equal(t, messages, got.Messages)
	})

	t.Run("responds with nothing when the server returns no choices", func(t *testing.T) {
		var got sent
		serveEndpoint(t, http.StatusOK, `{"choices":[]}`, &got)

		reply, err := egllm.LLamaCPP("m").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.Empty(t, reply)
	})

	t.Run("errors on a failing status", func(t *testing.T) {
		var got sent
		serveEndpoint(t, http.StatusInternalServerError, `boom`, &got)

		_, err := egllm.LLamaCPP("m").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.Error(t, err)
	})

	t.Run("errors on an invalid response", func(t *testing.T) {
		var got sent
		serveEndpoint(t, http.StatusOK, `not json`, &got)

		_, err := egllm.LLamaCPP("m").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.Error(t, err)
	})

	t.Run("uses the endpoint given over the environment", func(t *testing.T) {
		var got sent
		serveEndpoint(t, http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"direct"}}]}`, &got)
		endpoint := os.Getenv(eg.EnvComputeLLMEndpoint)
		t.Setenv(eg.EnvComputeLLMEndpoint, "http://unreachable.invalid")

		reply, err := egllm.LLamaCPP("m").Endpoint(endpoint+"/").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.Equal(t, "direct", reply)
		require.Equal(t, "/v1/chat/completions", got.Path)
	})

	t.Run("does not mutate the receiver", func(t *testing.T) {
		base := egllm.LLamaCPP("m")
		base.Endpoint("http://other")
		t.Setenv(eg.EnvComputeLLMEndpoint, "http://env")
		_, err := base.Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.ErrorContains(t, err, "env")
	})

	t.Run("an explicit endpoint skips server management", func(t *testing.T) {
		var ran bool
		op := egllm.LLamaCPP("m").Endpoint("http://llm.internal").With(func(context.Context, wasieg.Op) error {
			ran = true
			return nil
		})
		require.NoError(t, op(t.Context(), nil))
		require.True(t, ran)
	})
}
