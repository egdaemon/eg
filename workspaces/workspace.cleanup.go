package workspaces

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
)

// Cleanup removes stale cache entries from the workspace cache directory:
//   - any folder with content older than 30 days is removed outright.
//   - only the 3 most recent wazero compilation cache entries are kept.
//   - only the 3 most recent .gen entries are kept.
func (c Context) Cleanup(ctx context.Context) {
	for path := range fsx.Find(c.CacheDir, fsx.MaxAge(30*24*time.Hour), fsx.Levels(8)).Each(ctx) {
		errorsx.Log(errorsx.Wrapf(os.RemoveAll(path), "cache cleanup: %s", path))
	}

	for path := range fsx.KeepNewestN(3, fsx.Find(c.CacheDirWazero, fsx.Levels(8))).Each(ctx) {
		errorsx.Log(errorsx.Wrapf(os.RemoveAll(path), "wazero cache cleanup: %s", path))
	}

	for path := range fsx.KeepNewestN(3, fsx.Find(filepath.Join(c.CacheDir, eg.DefaultModuleDirectory(), ".gen"), fsx.Levels(8))).Each(ctx) {
		errorsx.Log(errorsx.Wrapf(os.RemoveAll(path), "gen cache cleanup: %s", path))
	}
}
