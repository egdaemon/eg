package runners

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/fsnotify/fsnotify"
	"github.com/gofrs/uuid/v5"
)

// Follow streams the log of the workload uid, starting at offset, into dst as it is
// written (tail -f), calling flush after each write. if the workload hasn't started
// running yet it waits for its log to be created. returns nil once the workload
// completes (its running directory is removed) and all of its log has been written.
func (t SpoolDirs) Follow(ctx context.Context, uid uuid.UUID, offset int64, dst io.Writer, flush func()) (err error) {
	var (
		watcher    *fsnotify.Watcher
		logs       *os.File
		rundir     = t.Path(uid).Resolve()
		runtimedir = t.Path(uid).Resolve(workdirname, eg.RuntimeDirectory)
		logpath    = filepath.Join(runtimedir, DaemonLogName)
	)

	if watcher, err = fsnotify.NewWatcher(); err != nil {
		return errorsx.Wrap(err, "unable to watch logs")
	}
	defer func() { errorsx.Log(errorsx.Wrap(watcher.Close(), "failed to close fs watch")) }()

	// the running directory reports the workload starting and completing.
	if err = watcher.Add(t.Running); err != nil {
		return errorsx.Wrap(err, "unable to watch running directory")
	}

	// wait for the log to be created, watching the directories leading to it as they appear.
	for {
		if err = watchexisting(watcher, rundir, filepath.Join(rundir, workdirname), runtimedir); err != nil {
			return err
		}

		// check after watching; the log may have been created between checks.
		if logs, err = os.Open(logpath); err == nil {
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return errorsx.Wrap(err, "unable to open logs")
		}

		if _, ok := t.Locate(uid); !ok {
			return nil
		}

		select {
		case <-watcher.Events:
		case err = <-watcher.Errors:
			return errorsx.Wrap(err, "unable to watch logs")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	defer logs.Close()

	if _, err = logs.Seek(offset, io.SeekStart); err != nil {
		return errorsx.Wrap(err, "unable to seek logs")
	}

	copylogs := func() error {
		defer flush()
		_, err := io.Copy(dst, logs)
		return err
	}

	for {
		if err = copylogs(); err != nil {
			return errorsx.Wrap(err, "unable to write logs")
		}

		select {
		case evt := <-watcher.Events:
			// completion is detected via the running directory being removed. the log's own
			// removal isn't reported while we hold it open; the open fd remains readable.
			if evt.Name == rundir && evt.Has(fsnotify.Remove|fsnotify.Rename) {
				return errorsx.Wrap(copylogs(), "unable to write logs")
			}
		case err = <-watcher.Errors:
			return errorsx.Wrap(err, "unable to watch logs")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// watchexisting adds the directories to the watcher, in order, stopping at the first that doesn't exist yet.
func watchexisting(watcher *fsnotify.Watcher, dirs ...string) error {
	for _, dir := range dirs {
		if err := watcher.Add(dir); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return errorsx.Wrapf(err, "unable to watch %s", dir)
		}
	}

	return nil
}
