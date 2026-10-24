package runners

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/history"
)

// historyrecorder keeps the outcome of completed workloads locally so the runner can answer for them
// without the control plane.
type historyrecorder struct {
	db   *sql.DB
	logs string
}

// record the outcome of the workload and keep its log, the workload's directory is removed when it is discarded.
// then purges the outcomes that expired.
func (t historyrecorder) record(ctx context.Context, workload *Enqueued, cause error, duration time.Duration, logpath string) (err error) {
	now := time.Now()

	entry := history.Entry{
		ID:          workload.Id,
		Description: workload.Description,
		VcsURI:      workload.VcsUri,
		VcsCommit:   workload.VcsCommit,
		Result:      history.ResultFrom(cause),
		CreatedAt:   now,
		InitiatedAt: now.Add(-duration),
		CompletedAt: now,
		Duration:    int64(duration),
		ExpiresAt:   now.Add(history.DefaultTTL),
	}

	if entry.LogsPath, err = t.keeplogs(workload.Id, logpath); err != nil {
		return err
	}

	if err = history.EntryUpsert(ctx, t.db, entry).Scan(&entry); err != nil {
		return errorsx.Wrap(err, "unable to record the workload")
	}

	return history.Purge(ctx, t.db)
}

// keeplogs keeps the log of the workload beyond the workload's directory by linking it into the history
// directory, copying when linking isn't possible (e.g. the directories are on different filesystems).
func (t historyrecorder) keeplogs(id string, src string) (path string, err error) {
	if err = os.MkdirAll(t.logs, 0700); err != nil {
		return "", errorsx.Wrap(err, "unable to create the history logs directory")
	}

	path = filepath.Join(t.logs, filepath.Base(id)+".log")

	// a repeated completion replaces the earlier link.
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", errorsx.Wrap(err, "unable to replace the history logs")
	}

	if err = os.Link(src, path); err == nil {
		return path, nil
	}

	in, err := os.Open(src)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to open the logs")
	}
	defer in.Close()

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to create the history logs")
	}

	if _, err = io.Copy(out, in); err != nil {
		errorsx.Log(out.Close())
		return "", errorsx.Wrap(err, "unable to copy the logs")
	}

	return path, errorsx.Wrap(out.Close(), "unable to copy the logs")
}
