package eggithub

import (
	"context"
	"fmt"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// provides the version pattern based on a github commit.
func PatternVersion() string {
	c := eggit.EnvCommit()
	return c.StringReplace("r%git.commit.year%.%git.commit.month%.%git.commit.day%%git.hash.short%")
}

// replaces the substitution values within the pattern, resulting in the final resulting archive file's name.
func archiveName(pattern string) string {
	c := eggit.EnvCommit()
	return fmt.Sprintf("%s.tar.xz", c.StringReplace(pattern))
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

// ReleaseIdempotent to github, this is experimental. it will delete any
// release with the same version effectively replacing it. this is to make the function idempotent.
// WARNING: for local environments this assumes you've provided the token to the eg command.
// e.g.) GH_TOKEN="$(gh auth token)" eg compute local -e GH_TOKEN
// WARNING: for hosted environments: we've assumed the git auth access token for pulling the repository
// will work. this has not yet been validated. and likely needs permission updates.
func ReleaseIdempotent(patterns ...string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		c := eggit.EnvCommit()
		version := PatternVersion()

		runtime := shell.Runtime().Environ(
			"GH_TOKEN", egenv.String("", "EG_GIT_AUTH_ACCESS_TOKEN", "GH_TOKEN"),
		)

		return shell.Run(
			ctx,
			runtime.Newf("gh release delete -y %s", version).Lenient(true),
			runtime.Newf("gh release create --target %s %s %s", c.Hash.String(), version, strings.Join(patterns, " ")),
		)
	}
}

// Release to github, this is very experimental.
// WARNING: for local environments this assumes you've provided the token to the eg command.
// e.g.) GH_TOKEN="$(gh auth token)" eg compute local -e GH_TOKEN
// WARNING: for hosted environments: we've assumed the git auth access token for pulling the repository
// will work. this has not yet been validated. and likely needs permission updates.
func Release(patterns ...string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		c := eggit.EnvCommit()
		version := PatternVersion()

		runtime := shell.Runtime().Environ(
			"GH_TOKEN", egenv.String("", "EG_GIT_AUTH_ACCESS_TOKEN", "GH_TOKEN"),
		)

		if shell.Run(ctx, runtime.Newf("gh release view %s", version)) != nil {
			return shell.Run(
				ctx,
				runtime.Newf("gh release create --target %s %s %s", c.Hash.String(), version, strings.Join(patterns, " ")),
			)
		}

		return eg.Sequential(
			Upload(version, patterns...),
		)(ctx, o)
	}
}

// Upload an asset to a github release, this is very experimental.
// WARNING: for local environments this assumes you've provided the token to the eg command.
// e.g.) GH_TOKEN="$(gh auth token)" eg compute local -e GH_TOKEN
// WARNING: for hosted environments: we've assumed the git auth access token for pulling the repository
// will work. this has not yet been validated. and likely needs permission updates.
// Usage:
//
//	eggithub.Upload(eggithub.PatternVersion(), "foo.txt", "bar.txt")
func Upload(release string, patterns ...string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		runtime := shell.Runtime().Environ(
			"GH_TOKEN", egenv.String("", "EG_GIT_AUTH_ACCESS_TOKEN", "GH_TOKEN"),
		)

		return shell.Run(
			ctx,
			runtime.Newf("gh release upload --clobber %s %s", release, strings.Join(patterns, " ")),
		)
	}
}
