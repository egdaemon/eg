package fsx

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/timex"
)

// KeepNewestN consumes s and yields all but the N newest directories.
// A directory's age is determined by the oldest entry found within it.
// Useful for pruning old entries while retaining a fixed number of recent ones.
func KeepNewestN(n int, s iterx.Seq[string]) iterx.Seq[string] {
	return iterx.New(func(ctx context.Context, yield func(string) bool) error {
		type entry struct {
			path    string
			oldest  int64
		}

		var entries []entry
		for path := range s.Each(ctx) {
			oldest := timex.Inf()

			info, err := os.Stat(path)
			if err != nil {
				return err
			}

			if !info.IsDir() {
				if t := info.ModTime(); t.Before(oldest) {
					oldest = t
				}
			} else {
				err = fs.WalkDir(os.DirFS(path), ".", func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil
					}

					select {
					case <-ctx.Done():
						return fs.SkipAll
					default:
					}

					info, err := d.Info()
					if err != nil {
						return nil
					}

					if t := info.ModTime(); t.Before(oldest) {
						oldest = t
					}

					return nil
				})

				if err != nil {
					return err
				}
			}

			entries = append(entries, entry{path: path, oldest: oldest.UnixNano()})
		}

		if err := s.Err(); err != nil {
			return err
		}

		// sort newest-first: the directory whose oldest file is most recent comes first
		slices.SortFunc(entries, func(a, b entry) int {
			return int(b.oldest - a.oldest)
		})

		for _, e := range entries[min(n, len(entries)):] {
			if !yield(filepath.Clean(e.path)) {
				return nil
			}
		}

		return nil
	})
}
