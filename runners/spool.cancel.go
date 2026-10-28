package runners

import (
	"context"
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

// watchcancel derives a context from ctx that is cancelled with ErrCancelled once the workload in
// dir is marked cancelled (see SpoolDirs.Cancel). the returned function must be called to release the watch.
func watchcancel(ctx context.Context, dir string) (context.Context, context.CancelFunc, error) {
	wctx, cancel := context.WithCancelCause(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		cancel(nil)
		return ctx, func() {}, errors.Wrap(err, "unable to create cancellation watcher")
	}

	// watch the directory, the marker doesn't exist yet.
	if err = watcher.Add(dir); err != nil {
		cancel(nil)
		errorsx.Log(watcher.Close())
		return ctx, func() {}, errors.Wrap(err, "unable to watch workload directory")
	}

	// the marker may have been created before the watch was established.
	if IsCancelled(dir) {
		cancel(ErrCancelled)
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case <-wctx.Done():
				return
			case evt, ok := <-watcher.Events:
				if !ok {
					return
				}

				if evt.Op.Has(fsnotify.Create) && filepath.Base(evt.Name) == cancelmarker {
					log.Println("cancellation requested", dir)
					cancel(ErrCancelled)
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				log.Println("cancellation watcher error", err)
			}
		}
	}()

	return wctx, func() { cancel(nil) }, nil
}
