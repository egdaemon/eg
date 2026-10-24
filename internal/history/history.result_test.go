package history_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/history"
	"github.com/stretchr/testify/require"
)

func TestResultFrom(t *testing.T) {
	require.Equal(t, history.ResultCompleted, history.ResultFrom(nil))
	require.Equal(t, history.ResultCancelled, history.ResultFrom(context.Canceled))
	require.Equal(t, history.ResultFailure, history.ResultFrom(errors.New("boom")))
}
