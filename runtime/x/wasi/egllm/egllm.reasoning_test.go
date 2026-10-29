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

func TestLLamaCPPReasoningBudget(t *testing.T) {
	serve := func(t *testing.T, body *map[string]any) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, body))

			_, err = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"reply"}}]}`))
			require.NoError(t, err)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("sends the budget with every request", func(t *testing.T) {
		var body map[string]any
		srv := serve(t, &body)

		got, err := egllm.LLamaCPP("m").Endpoint(srv.URL).ReasoningBudget(2048).Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.Equal(t, "reply", got)
		require.EqualValues(t, 2048, body["reasoning_budget_tokens"])
		require.Equal(t, "m", body["model"])
	})

	t.Run("leaves it to the server by default", func(t *testing.T) {
		var body map[string]any
		srv := serve(t, &body)

		_, err := egllm.LLamaCPP("m").Endpoint(srv.URL).Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.NotContains(t, body, "reasoning_budget_tokens")
		require.NotContains(t, body, "chat_template_kwargs")
	})

	t.Run("sends the effort to the chat template", func(t *testing.T) {
		var body map[string]any
		srv := serve(t, &body)

		_, err := egllm.LLamaCPP("m").Endpoint(srv.URL).ReasoningEffort("medium").Do(t.Context(), egllm.Message{Role: "user", Content: "a"})
		require.NoError(t, err)
		require.Equal(t, map[string]any{"reasoning_effort": "medium"}, body["chat_template_kwargs"])
	})
}
