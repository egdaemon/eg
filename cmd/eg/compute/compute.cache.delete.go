package compute

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/libp2px"
)

type cacheDelete struct {
	cacheoptions `embed:""`
	Paths        []string `arg:"" name:"paths" help:"paths relative to the cache root to remove"`
}

func (t cacheDelete) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, find libp2px.MDNSLookup) (err error) {
	cleaned, err := cacheDeletePaths(t.Paths...)
	if err != nil {
		return err
	}

	return t.cacheoptions.run(
		gctx, tlsc, ident, find,
		"cache/delete",
		fmt.Sprintf("cache delete - %s", strings.Join(cleaned, ", ")),
		[]string{envx.Format(cacheEnvDeletePaths, strings.Join(cleaned, string(filepath.ListSeparator)))},
	)
}

// cacheDeletePaths cleans the paths, rejecting any that are not within the cache root.
func cacheDeletePaths(paths ...string) ([]string, error) {
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, ok := fsx.Relative(".", p)
		if !ok || rel == "." {
			return nil, fmt.Errorf("path must be relative to and within the cache: %s", p)
		}

		if strings.ContainsRune(rel, filepath.ListSeparator) {
			return nil, fmt.Errorf("path must not contain %q: %s", filepath.ListSeparator, p)
		}

		cleaned = append(cleaned, rel)
	}

	return cleaned, nil
}
