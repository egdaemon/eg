// Package buildinfo describes the build of the running binary. it is separate from cmdopts so packages
// cmdopts depends on, transitively, can use it.
package buildinfo

import (
	"runtime/debug"
	"strconv"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/go-git/go-git/v6/plumbing"
)

// set at build time with -ldflags -X.
var (
	Treeish          = ""
	TreeishTimestamp = "" // RFC3339 commit time, used when vcs.time is unavailable (-buildvcs=false).
)

func Info() (_ string, err error) {
	var (
		ok    bool
		info  *debug.BuildInfo
		ts    time.Time
		id    string
		dirty string
	)

	if info, ok = debug.ReadBuildInfo(); !ok {
		return "", errorsx.Errorf("unable to read build info")
	}

	for _, v := range info.Settings {
		switch v.Key {
		case "vcs.modified":
			var (
				_dirty bool
			)
			if _dirty, err = strconv.ParseBool(v.Value); err != nil {
				return "", err
			}

			if _dirty {
				dirty = "dirty"
			}
		case "vcs.revision":
			id = v.Value
		case "vcs.time":
			if ts, err = time.Parse(time.RFC3339, v.Value); err != nil {
				return "", err
			}
		default:
			debugx.Printf("build.%s.%s\n", v.Key, v.Value)
		}
	}

	if ts.IsZero() && stringsx.Present(TreeishTimestamp) {
		if ts, err = time.Parse(time.RFC3339, TreeishTimestamp); err != nil {
			return "", err
		}
	}

	return stringsx.Join(".", slicesx.Filter(stringsx.Present, info.Main.Path, ts.Format("2006-01-02"), stringsx.DefaultIfBlank(id, Treeish), dirty)...), nil
}

// Commit returns the commit of the build, or the git zero commit when missing (i.e. `go run`).
func Commit() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, v := range info.Settings {
			if v.Key == "vcs.revision" && stringsx.Present(v.Value) {
				return v.Value
			}
		}
	}

	return stringsx.DefaultIfBlank(Treeish, plumbing.ZeroHash.String())
}

func Safe() string {
	return langx.FirstNonZero(errorsx.Zero(Info()), "(devel)")
}

func ModPath() string {
	info, _ := debug.ReadBuildInfo()
	return info.Main.Path
}
