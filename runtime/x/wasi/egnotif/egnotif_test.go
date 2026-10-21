package egnotif_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egnotif"
	"github.com/stretchr/testify/require"
)

type capture struct {
	got egnotif.Context
	err error
}

func (t *capture) Send(ctx context.Context, v egnotif.Context) error {
	t.got = v
	return t.err
}

type blocking struct{}

func (blocking) Send(ctx context.Context, v egnotif.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestNotifier(t *testing.T) {
	t.Run("completed returns nil and renders the completed template", func(t *testing.T) {
		t.Setenv(eg.EnvComputeRunID, "00000000-0000-0000-0000-000000000000")
		c := &capture{}
		require.NoError(t, egnotif.New(c, egnotif.Options().Completed("done {{.RunID}}")...).Fatalln(t.Context(), nil))
		require.False(t, c.got.Failed())
		msg, err := c.got.Message()
		require.NoError(t, err)
		require.Equal(t, "done 00000000-0000-0000-0000-000000000000", msg)
	})

	t.Run("failure returns the cause and renders the failure template", func(t *testing.T) {
		c := &capture{}
		cause := errors.New("boom")
		err := egnotif.New(c, egnotif.Options().Failure("failed: {{.Cause}}")...).Fatalln(t.Context(), cause)
		require.ErrorIs(t, err, cause)
		require.True(t, c.got.Failed())
		msg, err := c.got.Message()
		require.NoError(t, err)
		require.Equal(t, "failed: boom", msg)
	})

	t.Run("defaults are used when no options are provided", func(t *testing.T) {
		c := &capture{}
		require.NoError(t, egnotif.New(c).Fatalln(t.Context(), nil))
		require.Equal(t, egnotif.DefaultFailure, c.got.Failure)
		require.Equal(t, egnotif.DefaultCompleted, c.got.Completed)
		_, err := c.got.Message()
		require.NoError(t, err)
	})

	t.Run("delivery errors are joined with the cause", func(t *testing.T) {
		cause, derr := errors.New("boom"), errors.New("delivery")
		err := egnotif.New(&capture{err: derr}).Fatalln(t.Context(), cause)
		require.ErrorIs(t, err, cause)
		require.ErrorIs(t, err, derr)
	})

	t.Run("delivery errors are returned when the run completed", func(t *testing.T) {
		derr := errors.New("delivery")
		require.ErrorIs(t, egnotif.New(&capture{err: derr}).Fatalln(t.Context(), nil), derr)
	})

	t.Run("timeout bounds delivery", func(t *testing.T) {
		ts := time.Now()
		err := egnotif.New(blocking{}, egnotif.Options().Timeout(50*time.Millisecond)...).Fatalln(t.Context(), nil)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Less(t, time.Since(ts), 5*time.Second)
	})

	t.Run("initiated is provided by the environment", func(t *testing.T) {
		c := &capture{}
		require.NoError(t, egnotif.New(c).Fatalln(t.Context(), nil))
		require.False(t, c.got.Initiated.IsZero())
		require.GreaterOrEqual(t, c.got.Elapsed, time.Duration(0))
	})
}
