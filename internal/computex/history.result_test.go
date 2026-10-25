package computex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/computex"
	"github.com/stretchr/testify/require"
)

func TestResultFrom(t *testing.T) {
	require.Equal(t, computex.ResultCompleted, computex.ResultFrom(nil))
	require.Equal(t, computex.ResultCancelled, computex.ResultFrom(context.Canceled))
	require.Equal(t, computex.ResultFailure, computex.ResultFrom(errors.New("boom")))
}
