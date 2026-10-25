// Package history records the workloads a runner completed so the runner can answer for their
// result and logs without the control plane. records expire, see DefaultTTL.
package computex

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/gofrs/uuid/v5"
)

// DefaultTTL is how long a completed workload is remembered.
const DefaultTTL = 2 * time.Hour

// values of Entry.Result, they match the control plane's.
const (
	ResultUnknown   int32 = 0
	ResultFailure   int32 = 1
	ResultCompleted int32 = 2
	ResultCancelled int32 = 3
)

// ResultFrom determines the result of a workload from the cause it ended with.
func ResultFrom(cause error) int32 {
	switch {
	case cause == nil:
		return ResultCompleted
	case errors.Is(cause, context.Canceled):
		return ResultCancelled
	default:
		return ResultFailure
	}
}

// Purge deletes the outcomes that have expired along with their logs.
func Purge(ctx context.Context, q sqlx.Queryer) error {
	var expired []Entry

	if err := sqlx.ScanInto(EntryPurgeExpired(ctx, q), &expired); err != nil {
		return errorsx.Wrap(err, "unable to purge expired history")
	}

	for _, e := range expired {
		if e.LogsPath != "" {
			errorsx.Log(errorsx.Wrap(os.Remove(e.LogsPath), "unable to remove expired history logs"))
		}
	}

	return nil
}

// EntryOptionTestDefaults populates an entry with representative values and a unique id,
// intended for use with langx.Clone in tests.
func EntryOptionTestDefaults(e *Entry) {
	now := time.Now()
	e.ID = uuid.Must(uuid.NewV7()).String()
	e.Description = "workload - " + e.ID
	e.Result = ResultCompleted
	e.CreatedAt = now
	e.InitiatedAt = now.Add(-time.Minute)
	e.CompletedAt = now
	e.Duration = int64(time.Minute)
	e.ExpiresAt = now.Add(DefaultTTL)
}

// EntryOptionID sets the entry's id.
func EntryOptionID(id string) func(*Entry) {
	return func(e *Entry) {
		e.ID = id
	}
}
