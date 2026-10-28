package runners

import (
	"io"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/workspaces"
)

// cacheanalytics copies the analytics database of a successful workload into its cache directory when the workload
// requested it (see eg.AnalyticsCacheRequest), making it available to the next run of the workload.
func cacheanalytics(ws workspaces.Context, workload string, analyticspath string) (err error) {
	if !fsx.FileExists(filepath.Join(ws.RuntimeDir, eg.AnalyticsCacheRequest)) {
		return nil
	}

	src, err := os.Open(analyticspath)
	if err != nil {
		return errorsx.Wrap(err, "unable to open the analytics database")
	}
	defer src.Close()

	path := filepath.Join(ws.CacheDir, eg.AnalyticsCachePath(workload))
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errorsx.Wrap(err, "unable to create the cache directory")
	}

	dst, err := os.CreateTemp(filepath.Dir(path), "cache.*.db")
	if err != nil {
		return errorsx.Wrap(err, "unable to create the cache file")
	}
	defer os.Remove(dst.Name())
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return errorsx.Wrap(err, "unable to copy the analytics database")
	}

	if err = dst.Close(); err != nil {
		return errorsx.Wrap(err, "unable to write the analytics database")
	}

	return errorsx.Wrap(os.Rename(dst.Name(), path), "unable to cache the analytics database")
}
