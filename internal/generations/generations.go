// Package generations publishes sets of files for readers to use through a stable path. a generation is only
// reachable through the current symlink, which is replaced atomically, so readers never observe a partial one.
package generations

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
)

const (
	Current = "current"     // symlink to the generation in use.
	Past    = "generations" // holds the generations, the current one and the previous one.
)

// WriteFile writes the file such that readers see either the previous or the new content, never a partial file.
func WriteFile(path string, content []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

// Publish makes a new generation of files current. the generation is only reachable through the current symlink,
// which is replaced by renaming a new one over it, an atomic operation, once the generation is complete. readers
// using the path through current see either the previous or the new generation, never a partial one.
// all but the newest two generations are removed.
// build writes the files into the directory, when it returns false nothing is published.
func Publish(ctx context.Context, base string, chown func(path string) error, build func(dir string) (bool, error)) (bool, error) {
	generations := filepath.Join(base, Past)
	generation := filepath.Join(generations, strconv.FormatInt(time.Now().UnixNano(), 10)) // named with the time it was published.

	if err := fsx.MkDirs(0770, generation); err != nil {
		return false, errorsx.Wrapf(err, "unable to create %s", generation)
	}

	if ok, err := build(generation); err != nil || !ok {
		return false, errorsx.Compact(err, os.RemoveAll(generation))
	}

	entries, err := os.ReadDir(generation)
	if err != nil {
		return false, errorsx.Compact(errorsx.Wrapf(err, "unable to read %s", generation), os.RemoveAll(generation))
	}

	owned := []string{base, generations, generation}
	for _, e := range entries {
		owned = append(owned, filepath.Join(generation, e.Name()))
	}

	for _, path := range owned {
		if err = chown(path); err != nil {
			return false, errorsx.Compact(errorsx.Wrapf(err, "unable to give the user access to %s", path), os.RemoveAll(generation))
		}
	}

	if err = Link(filepath.Join(base, Current), filepath.Join(Past, filepath.Base(generation))); err != nil {
		return false, errorsx.Compact(errorsx.Wrap(err, "unable to replace the current generation"), os.RemoveAll(generation))
	}

	// keep the previous generation, a reader that resolved its path a moment ago still finishes reading it.
	return true, errorsx.Wrap(fsx.Prune(ctx, fsx.KeepNewestN(2, fsx.Find(generations, fsx.Levels(1)))), "unable to remove old generations")
}

// Link atomically creates or replaces the symlink at path pointing to target.
func Link(path, target string) error {
	tmp := path + ".tmp"
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Symlink(target, tmp); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}
