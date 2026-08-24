package eggithub

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// provides the version pattern based on a github commit.
// provides sequentially incrementing versions based on the commit dates.
// assuming you're generating commits on merge this will always move forward.
func PatternVersion() string {
	c := eggit.EnvCommit()
	return c.StringReplace("r%git.commit.year%.%git.commit.month%.%git.commit.day%%git.commit.unix%")
}

// replaces the substitution values within the pattern, resulting in the final resulting archive file's name.
func archiveName(pattern string) string {
	return eggit.EnvCommit().StringReplace(pattern)
}

// generate the github download url
func DownloadURL(pattern string) string {
	version := PatternVersion()
	archive := archiveName(pattern)
	canon := eggit.EnvCanonicalURI()                                                                     // git@github.com:egdaemon/eg.git
	canon = strings.ReplaceAll(canon, ".git", fmt.Sprintf("/releases/download/%s/%s", version, archive)) // git@github.com:egdaemon/eg/releases/download/%release%/%archive%
	canon = strings.ReplaceAll(canon, ":", "/")                                                          // git@github.com/egdaemon/eg/releases/download/%release%/%archive%
	canon = strings.ReplaceAll(canon, "git@", "https://")                                                // https://github.com:egdaemon/eg/releases/download/%release%/%archive%

	return canon
}

// Draft creates (or reuses) a draft release for PatternVersion and uploads
// the given patterns to it. Does not verify or publish the release — pair
// with Promote to publish once all expected assets have been uploaded.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
func Draft(patterns ...string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		c := eggit.EnvCommit()
		version := PatternVersion()

		runtime := shell.Runtime().Environ(
			_eg.EnvGithubToken, ffigit.Bearer(),
		)

		if shell.Run(ctx, runtime.Newf("gh release view %s", version)) != nil {
			return shell.Run(
				ctx,
				runtime.Newf("gh release create --draft --target %s %s %s", c.Hash.String(), version, strings.Join(patterns, " ")),
			)
		}

		return Upload(version, patterns...)(ctx, o)
	}
}

// assetMatch reports whether pattern (a glob against an asset's basename, e.g.
// "eg_*_amd64.deb") matches any non-blank line within assets, a
// newline-separated listing of a release's asset names.
func assetMatch(assets, pattern string) bool {
	for line := range strings.SplitSeq(assets, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if ok, _ := filepath.Match(pattern, line); ok {
			return true
		}
	}

	return false
}

// verifyAssets checks that every pattern has a corresponding entry in assets,
// a newline-separated listing of a release's asset names. When a pattern
// includes a directory component it is first resolved against the local
// filesystem so the error can name the specific missing file; a directory-less
// pattern (e.g. "eg_*_amd64.deb") is matched directly against the asset names.
// Returns an error naming the version and the first pattern or file found
// missing.
func verifyAssets(version, assets string, patterns ...string) error {
	for _, p := range patterns {
		if filepath.Dir(p) == "." {
			if !assetMatch(assets, p) {
				return fmt.Errorf("release %s missing asset matching pattern: %s\n%s", version, p, assets)
			}

			continue
		}

		matches, err := filepath.Glob(p)
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			return fmt.Errorf("release %s missing asset matching pattern: %s", version, p)
		}

		for _, m := range matches {
			if !assetMatch(assets, filepath.Base(m)) {
				return fmt.Errorf("release %s missing asset: %s\n%s", version, filepath.Base(m), assets)
			}
		}
	}

	return nil
}

// Promote a draft release to a full release, this is very experimental.
// uses PatternVersion to determine which draft release to promote, and verifies
// that every pattern is already present in the release's asset listing before
// publishing it.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
func Promote(patterns ...string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		var (
			path    = egenv.EphemeralDirectory("eg.github.release.assets")
			version = PatternVersion()
		)

		runtime := shell.Runtime().Environ(
			_eg.EnvGithubToken, ffigit.Bearer(),
		)

		if len(patterns) > 0 {
			err := shell.Run(
				ctx,
				runtime.Newf(`gh release view %s --json assets --jq ".assets[].name" > %s`, version, path),
			)
			if err != nil {
				return err
			}

			lines, err := fsx.String(path)
			if err != nil {
				return err
			}

			if err := verifyAssets(version, lines, patterns...); err != nil {
				return err
			}
		}

		return shell.Run(
			ctx,
			runtime.Newf("gh release edit %s --draft=false", version),
		)
	}
}

// Release to github, this is very experimental.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
func Release(patterns ...string) eg.OpFn {
	return eg.Sequential(
		Draft(patterns...),
		Promote(),
	)
}

// Upload an asset to a github release, this is very experimental. uploads into
// whatever release currently exists for the given version (typically a draft
// created by Draft), and does not publish it.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
// Usage:
//
//	eggithub.Upload(eggithub.PatternVersion(), "foo.txt", "bar.txt")
func Upload(release string, patterns ...string) eg.OpFn {
	return UploadRuntime(shell.Runtime().Attempts(3).Timeout(time.Duration(len(patterns))*shell.DefaultTimeout), release, patterns...)
}

func UploadRuntime(rt shell.Command, release string, patterns ...string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		runtime := rt.Environ(
			_eg.EnvGithubToken, ffigit.Bearer(),
		)

		return shell.Run(
			ctx,
			runtime.Newf("gh release upload --clobber %s %s", release, strings.Join(patterns, " ")),
		)
	}
}
