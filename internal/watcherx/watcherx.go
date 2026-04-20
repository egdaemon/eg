package watcherx

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/radovskyb/watcher"
)

// Proxy watches src recursively and mirrors file system events into dst.
// It blocks until ctx is cancelled or a fatal error occurs.
func Proxy(ctx context.Context, src, dst string, interval time.Duration) error {
	w := watcher.New()
	w.FilterOps(watcher.Create, watcher.Write, watcher.Remove, watcher.Rename, watcher.Move)

	if err := w.AddRecursive(src); err != nil {
		return err
	}

	errc := make(chan error, 1)
	go func() {
		errc <- w.Start(interval)
	}()

	for {
		select {
		case <-ctx.Done():
			w.Close()
			return ctx.Err()
		case err := <-errc:
			return err
		case err := <-w.Error:
			return err
		case <-w.Closed:
			return nil
		case event := <-w.Event:
			if err := applyEvent(src, dst, event); err != nil {
				errorsx.Log(errorsx.Wrap(err, "unable to apply file system event"))
			}
		}
	}
}

func applyEvent(src, dst string, event watcher.Event) error {
	rel, err := filepath.Rel(src, event.Path)
	if err != nil {
		return err
	}
	target := filepath.Join(dst, rel)

	switch event.Op {
	case watcher.Create, watcher.Write:
		return touchFile(target)
	case watcher.Remove, watcher.Rename, watcher.Move:
		return touchFile(filepath.Dir(target))
	}
	return nil
}

func touchFile(path string) error {
	return os.Chtimes(path, time.Now(), time.Time{})
}
