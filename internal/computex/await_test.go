package computex_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestAwait(t *testing.T) {
	t.Run("reports the result from the supplied source", func(t *testing.T) {
		find := func(ctx context.Context, id string) (*compute.History, error) {
			return &compute.History{Id: id, Result: computex.ResultFailure}, nil
		}
		require.ErrorContains(t, computex.Await(t.Context(), "w1", find), "workload w1 failed")
	})

	t.Run("a failure to retrieve the record is unavailable", func(t *testing.T) {
		find := func(ctx context.Context, id string) (*compute.History, error) {
			return nil, errors.New("boom")
		}
		require.ErrorAs(t, computex.Await(t.Context(), "w1", find), &computex.Unavailable{})
	})

	t.Run("retries a record that isn't there yet until the context ends", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer done()
		find := func(ctx context.Context, id string) (*compute.History, error) {
			return nil, httpx.Error{Code: http.StatusNotFound}
		}
		require.ErrorIs(t, computex.Await(ctx, "w1", find), context.DeadlineExceeded)
	})
}

func TestAwaitFallback(t *testing.T) {
	broken := func(ctx context.Context, id string) (*compute.History, error) { return nil, errors.New("boom") }
	completed := func(ctx context.Context, id string) (*compute.History, error) {
		return &compute.History{Id: id, Result: computex.ResultCompleted}, nil
	}
	failed := func(ctx context.Context, id string) (*compute.History, error) {
		return &compute.History{Id: id, Result: computex.ResultFailure}, nil
	}

	t.Run("falls back until one works", func(t *testing.T) {
		require.NoError(t, computex.Await(t.Context(), "w1", broken, broken, completed))
	})

	t.Run("a workload failure does not fall back", func(t *testing.T) {
		require.ErrorContains(t, computex.Await(t.Context(), "w1", failed, completed), "workload w1 failed")
	})

	t.Run("the last error is returned when none work", func(t *testing.T) {
		require.ErrorAs(t, computex.Await(t.Context(), "w1", broken, broken), &computex.Unavailable{})
	})

	t.Run("requires a source", func(t *testing.T) {
		require.Error(t, computex.Await(t.Context(), "w1"))
	})
}

func TestLogs(t *testing.T) {
	t.Run("writes the stored log", func(t *testing.T) {
		fetch := func(ctx context.Context, id string, dst io.Writer) error {
			_, err := dst.Write([]byte(id))
			return err
		}
		var b writer
		require.NoError(t, computex.Logs(t.Context(), "w1", &b, fetch))
		require.Equal(t, "w1", string(b))
	})

	t.Run("unavailable logs are an error", func(t *testing.T) {
		fetch := func(ctx context.Context, id string, dst io.Writer) error { return errors.New("boom") }
		require.ErrorContains(t, computex.Logs(t.Context(), "w1", io.Discard, fetch), "stored workload logs unavailable")
	})
}

type writer []byte

func (t *writer) Write(p []byte) (int, error) {
	*t = append(*t, p...)
	return len(p), nil
}
