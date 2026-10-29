package runners

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/errorsx"
)

// historyrecorder keeps the outcome of completed workloads locally so the runner can answer for them
// without the control plane.
type historyrecorder struct {
	db   *sql.DB
	logs string
}

// record the outcome of the workload and keep its log, the workload's directory is removed when it is discarded.
// then purges the outcomes that expired.
func (t historyrecorder) record(ctx context.Context, workload *computeapi.Enqueued, cause error, duration time.Duration, logpath string) (err error) {
	now := time.Now()

	entry := computex.Entry{
		ID:          workload.Id,
		Description: workload.Description,
		VcsURI:      workload.VcsUri,
		VcsCommit:   workload.VcsCommit,
		Result:      computex.ResultFrom(cause),
		CreatedAt:   now,
		InitiatedAt: now.Add(-duration),
		CompletedAt: now,
		Duration:    int64(duration),
		ExpiresAt:   now.Add(computex.DefaultTTL),
	}

	if entry.LogsPath, err = t.keeplogs(workload.Id, logpath); err != nil {
		return err
	}

	// the analytics live beside the log in the workload's runtime directory, a workload without them is still recorded.
	if entry.AnalyticsPath, err = t.keepanalytics(workload.Id, filepath.Join(filepath.Dir(logpath), DaemonAnalyticsName)); errors.Is(err, os.ErrNotExist) {
		entry.AnalyticsPath = ""
	} else if err != nil {
		return err
	}

	if err = computex.EntryUpsert(ctx, t.db, entry).Scan(&entry); err != nil {
		return errorsx.Wrap(err, "unable to record the workload")
	}

	return computex.Purge(ctx, t.db)
}

// keeplogs keeps the log of the workload beyond the workload's directory by linking it into the history
// directory, copying when linking isn't possible (e.g. the directories are on different filesystems).
func (t historyrecorder) keeplogs(id string, src string) (path string, err error) {
	return t.keep(src, filepath.Join(t.logs, filepath.Base(id)+".log"), "logs")
}

// keepanalytics keeps the analytics of the workload beyond the workload's directory, in the same manner as keeplogs.
func (t historyrecorder) keepanalytics(id string, src string) (path string, err error) {
	return t.keep(src, filepath.Join(t.logs, filepath.Base(id)+".analytics.db"), "analytics")
}

func (t historyrecorder) keep(src string, path string, kind string) (_ string, err error) {
	if err = os.MkdirAll(t.logs, 0700); err != nil {
		return "", errorsx.Wrapf(err, "unable to create the history %s directory", kind)
	}

	// a repeated completion replaces the earlier link.
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", errorsx.Wrapf(err, "unable to replace the history %s", kind)
	}

	if err = os.Link(src, path); err == nil {
		return path, nil
	}

	in, err := os.Open(src)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to open the %s", kind)
	}
	defer in.Close()

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to create the history %s", kind)
	}

	if _, err = io.Copy(out, in); err != nil {
		errorsx.Log(out.Close())
		return "", errorsx.Wrapf(err, "unable to copy the %s", kind)
	}

	return path, errorsx.Wrapf(out.Close(), "unable to copy the %s", kind)
}
