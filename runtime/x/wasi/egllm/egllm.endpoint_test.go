package egllm_test

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestEndpoint(t *testing.T) {
	t.Run("defaults to llama.cpp", func(t *testing.T) {
		t.Setenv(eg.EnvComputeLLMEndpoint, "")
		require.Equal(t, "unix:///run/llama-server/llama.sock", egllm.Endpoint())
		require.Equal(t, egllm.DefaultEndpoint, egllm.Endpoint())
	})

	t.Run("is overridden by the environment", func(t *testing.T) {
		t.Setenv(eg.EnvComputeLLMEndpoint, "http://llm.internal:9090")
		require.Equal(t, "http://llm.internal:9090", egllm.Endpoint())
	})

	t.Run("drops trailing slashes", func(t *testing.T) {
		t.Setenv(eg.EnvComputeLLMEndpoint, "http://llm.internal:9090//")
		require.Equal(t, "http://llm.internal:9090", egllm.Endpoint())
	})
}

func TestNew(t *testing.T) {
	t.Run("uses the default endpoint", func(t *testing.T) {
		t.Setenv(eg.EnvComputeLLMEndpoint, "")
		require.Equal(t, egllm.DefaultEndpoint, egllm.New().Host())
	})

	t.Run("uses the overridden endpoint", func(t *testing.T) {
		t.Setenv(eg.EnvComputeLLMEndpoint, "http://llm.internal:9090")
		require.Equal(t, "http://llm.internal:9090", egllm.New().Host())
	})
}
