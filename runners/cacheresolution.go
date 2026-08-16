package runners

import (
	"context"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"sync"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/pkg/errors"
)

// CacheResolution claims/releases fan-out lock keys ("buckets") for a repo,
// backed by marker directories under blockeddir. A claimed bucket is the
// caller's license to use that bucket as a cache/volume directory segment
// for the duration of the run.
type CacheResolution struct {
	blockeddir   string
	queueddir    string
	defaultPerms fs.FileMode
	renamemux    *sync.Mutex
	buckets      int
}

// NewCacheResolution builds a CacheResolution backed by dirs' Blocked/Queued
// directories, sharing dirs' rename mutex so lock claims stay serialized
// with the spool's own Dequeue renames. buckets caps how many candidates
// Claim will try from a candidate sequence before giving up and parking.
func NewCacheResolution(dirs SpoolDirs, buckets int) CacheResolution {
	return CacheResolution{blockeddir: dirs.Blocked, queueddir: dirs.Queued, defaultPerms: dirs.defaultPerms, renamemux: dirs.renamemux, buckets: buckets}
}

// tryClaim claims bucket with no side effects on dir. nil = claimed,
// ErrRepoBlocked = already claimed by someone else.
func (c CacheResolution) tryClaim(bucket string) error {
	if err := os.Mkdir(filepath.Join(c.blockeddir, bucket), c.defaultPerms); err == nil {
		return nil
	} else if os.IsExist(err) {
		return ErrRepoBlocked
	} else {
		return err
	}
}

// park moves dir under Blocked/<bucket>, assumes bucket is already claimed.
func (c CacheResolution) park(bucket, dir string) error {
	return os.Rename(dir, filepath.Join(c.blockeddir, bucket, filepath.Base(dir)))
}

// Claim tries candidates from candidates in order, up to c.buckets of them,
// claiming the first unclaimed one. If every candidate tried is already
// claimed, dir (the spool run directory for this workload instance) is
// parked under the last one tried, to be replayed once that bucket is
// Released, and ErrRepoBlocked is returned.
func (c CacheResolution) Claim(ctx context.Context, candidates iter.Seq[string], dir string) (bucket string, err error) {
	c.renamemux.Lock()
	defer c.renamemux.Unlock()

	var (
		last  string
		tried int
	)
	for candidate := range candidates {
		if tried >= c.buckets {
			break
		}
		tried++

		last = candidate
		if err = c.tryClaim(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, ErrRepoBlocked) {
			return "", err
		}
	}

	if err = c.park(last, dir); err != nil {
		return "", err
	}
	return "", ErrRepoBlocked
}

// Release frees bucket: anything parked under Blocked/<bucket> is moved back
// to Queued, then the marker directory is removed. Safe to call on a bucket
// that was never claimed (no-op).
func (c CacheResolution) Release(bucket string) (err error) {
	c.renamemux.Lock()
	defer c.renamemux.Unlock()

	blockeddir := filepath.Join(c.blockeddir, bucket)
	if err = fs.WalkDir(os.DirFS(blockeddir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if err := os.Rename(filepath.Join(blockeddir, d.Name()), filepath.Join(c.queueddir, d.Name())); err != nil {
			return err
		}
		return fs.SkipDir
	}); fsx.ErrIsNotExist(err) != nil {
		return nil
	} else if err != nil {
		return err
	}

	return os.Remove(blockeddir)
}
