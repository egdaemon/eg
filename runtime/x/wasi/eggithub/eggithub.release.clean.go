package eggithub

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// defaultReleaseKeep is how many published releases ReleaseClean keeps, unless told otherwise.
const defaultReleaseKeep = 10

// staleReleases returns the tags of the published (non draft) releases within listing, the json output of
// `gh release list --json tagName,isDraft,createdAt`, that are not among the keep newest.
func staleReleases(listing string, keep int) ([]string, error) {
	var releases []struct {
		Tag       string    `json:"tagName"`
		Draft     bool      `json:"isDraft"`
		CreatedAt time.Time `json:"createdAt"`
	}

	if err := json.Unmarshal([]byte(listing), &releases); err != nil {
		return nil, err
	}

	published := releases[:0]
	for _, r := range releases {
		if !r.Draft {
			published = append(published, r)
		}
	}

	if len(published) <= keep {
		return nil, nil
	}

	sort.SliceStable(published, func(i, j int) bool {
		return published[i].CreatedAt.After(published[j].CreatedAt)
	})

	var tags []string
	for _, r := range published[keep:] {
		tags = append(tags, r.Tag)
	}

	return tags, nil
}

// ReleaseClean deletes the published releases beyond the keep newest, the counterpart to DraftClean
// which handles drafts. drafts are never touched or counted, and the git tags of deleted releases are kept.
// a keep of zero or less is ten.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
func ReleaseClean(keep int) eg.OpFn {
	if keep <= 0 {
		keep = defaultReleaseKeep
	}

	return func(ctx context.Context, o eg.Op) error {
		return releaseclean(ctx, keep)
	}
}

func releaseclean(ctx context.Context, keep int) error {
	var (
		path    = egenv.DirectoryTemp("eg.github.release.releases")
		runtime = shell.Runtime().Environ(_eg.EnvGithubToken, ffigit.Bearer())
	)

	err := shell.Run(
		ctx,
		runtime.Newf("gh release list --limit 200 --json tagName,isDraft,createdAt > %s", path),
	)
	if err != nil {
		return err
	}

	listing, err := fsx.String(path)
	if err != nil {
		return err
	}

	tags, err := staleReleases(listing, keep)
	if err != nil {
		return fmt.Errorf("unable to parse the release listing: %w", err)
	}

	for _, tag := range tags {
		if err := shell.Run(ctx, runtime.Newf("gh release delete %s --yes", tag)); err != nil {
			return err
		}
	}

	return nil
}
