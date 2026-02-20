package fsx

import (
	"context"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/timex"
)

type filterConfig struct {
	maxAge time.Duration
	levels int
}

// FilterOption configures the behavior of Find.
type FilterOption func(*filterConfig)

// MaxAge includes directories that contain at least one entry older than d.
func MaxAge(d time.Duration) FilterOption {
	return func(c *filterConfig) {
		c.maxAge = d
	}
}

// Levels limits how many directory levels deep to scan inside each candidate
// when determining its effective modification time.
func Levels(n int) FilterOption {
	return func(c *filterConfig) {
		c.levels = n
	}
}

// Find iterates over the immediate subdirectories of root, yielding those that
// satisfy all provided filter options. A directory's age is determined by the
// oldest entry found within it (scanned up to the configured depth).
// The root directory itself is never yielded.
func Find(root string, options ...FilterOption) iterx.Seq[string] {
	cfg := &filterConfig{
		maxAge: 0,
		levels: math.MaxInt,
	}
	for _, opt := range options {
		opt(cfg)
	}

	return iterx.New(func(ctx context.Context, yield func(string) bool) error {
		now := time.Now()

		var (
			curName   string
			curOldest time.Time
		)

		check := func() bool {
			if curName == "" {
				return false
			}
			if now.Sub(curOldest) < cfg.maxAge {
				return false
			}
			return true
		}

		err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return fs.SkipAll
			default:
			}

			if path == "." {
				return nil
			}

			topLevel, _, _ := strings.Cut(path, "/")
			depth := strings.Count(path, "/") + 1

			if d.IsDir() && depth >= cfg.levels {
				return fs.SkipDir
			}

			if topLevel != curName {
				if check() && !yield(filepath.Join(root, curName)) {
					return fs.SkipAll
				}
				curName = topLevel
				curOldest = timex.Inf()
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			if t := info.ModTime(); t.Before(curOldest) {
				curOldest = t
			}

			return nil
		})

		if check() {
			yield(filepath.Join(root, curName))
		}

		return errorsx.Wrap(err, "find: walking root directory")
	})
}
