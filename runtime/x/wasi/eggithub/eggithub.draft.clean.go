package eggithub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// defaultDraftTTL is how old a draft release has to be before DraftClean deletes it, unless told otherwise.
const defaultDraftTTL = 24 * time.Hour

// staleDrafts returns the tags of the draft releases within listing, the json output of
// `gh release list --json tagName,isDraft,createdAt`, that were created more than ttl before now.
func staleDrafts(listing string, now time.Time, ttl time.Duration) ([]string, error) {
	var releases []struct {
		Tag       string    `json:"tagName"`
		Draft     bool      `json:"isDraft"`
		CreatedAt time.Time `json:"createdAt"`
	}

	if err := json.Unmarshal([]byte(listing), &releases); err != nil {
		return nil, err
	}

	var tags []string
	for _, r := range releases {
		if r.Draft && now.Sub(r.CreatedAt) > ttl {
			tags = append(tags, r.Tag)
		}
	}

	return tags, nil
}

// DraftClean deletes the draft releases that are older than ttl, drafts left behind by releases that
// failed or were abandoned before Promote. recent drafts, including the one in progress, are kept.
// a ttl of zero or less is a day.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
func DraftClean(ttl time.Duration) eg.OpFn {
	if ttl <= 0 {
		ttl = defaultDraftTTL
	}

	return func(ctx context.Context, o eg.Op) error {
		return draftclean(ctx, ttl)
	}
}

func draftclean(ctx context.Context, ttl time.Duration) error {
	var (
		path    = egenv.DirectoryTemp("eg.github.release.drafts")
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

	tags, err := staleDrafts(listing, time.Now(), ttl)
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
