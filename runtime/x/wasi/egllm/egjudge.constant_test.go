package egllm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConstant(t *testing.T) {
	j := Constant("a", "b")

	r, err := j.Do(t.Context(), Message{Role: "user", Content: "one"})
	require.NoError(t, err)
	require.Equal(t, "a", r)

	r, err = j.Do(t.Context(), Message{Role: "user", Content: "two"})
	require.NoError(t, err)
	require.Equal(t, "b", r)

	_, err = j.Do(t.Context())
	require.Error(t, err)

	require.Len(t, j.Requests(), 3)
	require.Equal(t, "two", j.Requests()[1][0].Content)
}
