package egllm_test

import (
	"context"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
	"github.com/stretchr/testify/require"
)

func TestEcho(t *testing.T) {
	for _, tc := range []struct {
		name     string
		messages []egllm.Message
		expected string
	}{
		{
			name:     "none",
			expected: "",
		},
		{
			name:     "single",
			messages: []egllm.Message{{Role: "user", Content: "hello"}},
			expected: "-------------- user initiated --------------\nhello\n-------------- user completed --------------",
		},
		{
			name: "multiple",
			messages: []egllm.Message{
				{Role: "user", Content: "a"},
				{Role: "assistant", Content: "b"},
			},
			expected: "-------------- user initiated --------------\na\n-------------- user completed --------------\n" +
				"-------------- assistant initiated --------------\nb\n-------------- assistant completed --------------",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := egllm.Echo().Do(context.Background(), tc.messages...)
			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestEchoIsNotAServer(t *testing.T) {
	_, ok := egllm.Echo().(egllm.Server)
	require.False(t, ok)
	require.Implements(t, (*egllm.Server)(nil), egllm.LLamaCPP("m"))
	require.Implements(t, (*egllm.Judge)(nil), egllm.LLamaCPP("m"))
}
