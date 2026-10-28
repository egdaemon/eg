package egllm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestClaudeDo(t *testing.T) {
	type sent struct {
		Path      string
		Key       string
		Version   string
		Model     string          `json:"model"`
		MaxTokens int             `json:"max_tokens"`
		System    string          `json:"system"`
		Messages  []egllm.Message `json:"messages"`
	}

	serve := func(t *testing.T, status int, body string, got *sent) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, got))
			got.Path = r.URL.Path
			got.Key = r.Header.Get("x-api-key")
			got.Version = r.Header.Get("anthropic-version")

			w.WriteHeader(status)
			_, err = w.Write([]byte(body))
			require.NoError(t, err)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("posts to the messages api hoisting system messages into the system field", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusOK, `{"content":[{"type":"text","text":"re"},{"type":"tool_use"},{"type":"text","text":"ply"}]}`, &got)
		t.Setenv(egllm.EnvAnthropicAPIKey, "env-key")
		t.Setenv(egllm.EnvAnthropicBaseURL, srv.URL)

		reply, err := egllm.Claude("claude-model").Do(t.Context(),
			egllm.Message{Role: "system", Content: "rules"},
			egllm.Message{Role: "user", Content: "a"},
			egllm.Message{Role: "assistant", Content: "b"},
			egllm.Message{Role: "user", Content: "c"},
		)
		require.NoError(t, err)
		require.Equal(t, "reply", reply)
		require.Equal(t, "/v1/messages", got.Path)
		require.Equal(t, "env-key", got.Key)
		require.Equal(t, "2023-06-01", got.Version)
		require.Equal(t, "claude-model", got.Model)
		require.NotZero(t, got.MaxTokens)
		require.Equal(t, "rules", got.System)
		require.Equal(t, []egllm.Message{
			{Role: "user", Content: "a"},
			{Role: "assistant", Content: "b"},
			{Role: "user", Content: "c"},
		}, got.Messages)
	})

	t.Run("options override the environment", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusOK, `{"content":[{"type":"text","text":"ok"}]}`, &got)
		t.Setenv(egllm.EnvAnthropicAPIKey, "env-key")
		t.Setenv(egllm.EnvAnthropicBaseURL, "http://127.0.0.1:1")

		reply, err := egllm.Claude("m").APIKey("opt-key").Endpoint(srv.URL+"/").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.Equal(t, "ok", reply)
		require.Equal(t, "opt-key", got.Key)
	})

	t.Run("max tokens defaults and is configurable", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusOK, `{"content":[]}`, &got)
		msg := egllm.Message{Role: "user", Content: "a"}

		_, err := egllm.Claude("m").APIKey("k").Endpoint(srv.URL).Do(t.Context(), msg)
		require.NoError(t, err)
		require.Equal(t, 8192, got.MaxTokens)

		_, err = egllm.Claude("m").APIKey("k").Endpoint(srv.URL).MaxTokens(1234).Do(t.Context(), msg)
		require.NoError(t, err)
		require.Equal(t, 1234, got.MaxTokens)
	})

	t.Run("responds with nothing when there is no content", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusOK, `{"content":[]}`, &got)

		reply, err := egllm.Claude("m").APIKey("k").Endpoint(srv.URL).Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.Empty(t, reply)
	})

	t.Run("fails without leaking the key when the api errors", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusUnauthorized, `{"error":"nope"}`, &got)

		_, err := egllm.Claude("m").APIKey("secret-key").Endpoint(srv.URL).Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret-key")
	})

	t.Run("errors carry the reason given by the api", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusBadRequest, `{"type":"error","error":{"type":"invalid_request_error","message":"credit balance is too low"}}`, &got)

		_, err := egllm.Claude("m").APIKey("secret-key").Endpoint(srv.URL).Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		var cerr *egllm.ClaudeError
		require.ErrorAs(t, err, &cerr)
		require.Equal(t, http.StatusBadRequest, cerr.Status)
		require.Equal(t, "invalid_request_error", cerr.Type)
		require.Equal(t, "credit balance is too low", cerr.Message)
		require.Contains(t, err.Error(), "400 Bad Request")
		require.Contains(t, err.Error(), "invalid_request_error")
		require.Contains(t, err.Error(), "credit balance is too low")
		require.NotContains(t, err.Error(), "secret-key")
		require.NotNil(t, httpx.IsStatusError(err, http.StatusBadRequest))
		require.Nil(t, httpx.IsStatusError(err, http.StatusUnauthorized))
	})

	t.Run("errors fall back to the body when it is not an api error", func(t *testing.T) {
		var got sent
		srv := serve(t, http.StatusBadGateway, "  upstream down\n", &got)

		_, err := egllm.Claude("m").APIKey("k").Endpoint(srv.URL).Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		var cerr *egllm.ClaudeError
		require.ErrorAs(t, err, &cerr)
		require.Empty(t, cerr.Type)
		require.Equal(t, "upstream down", cerr.Message)
		require.Contains(t, err.Error(), "upstream down")
	})

	t.Run("fails without an api key", func(t *testing.T) {
		t.Setenv(egllm.EnvAnthropicAPIKey, "")

		_, err := egllm.Claude("m").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.Error(t, err)
	})

	t.Run("is a judge and a server", func(t *testing.T) {
		require.Implements(t, (*egllm.Judge)(nil), egllm.Claude("m"))
		require.Implements(t, (*egllm.Server)(nil), egllm.Claude("m"))
	})

	t.Run("with runs the operations in sequence", func(t *testing.T) {
		var order []int
		op := func(i int) eg.OpFn {
			return func(context.Context, eg.Op) error {
				order = append(order, i)
				return nil
			}
		}

		require.NoError(t, egllm.Claude("m").With(op(1), op(2), op(3))(t.Context(), egtest.Op()))
		require.Equal(t, []int{1, 2, 3}, order)
	})
}
