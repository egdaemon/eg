package eggithub

import (
	"context"
	"fmt"
	"strings"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type pullrequestopts struct {
	title string
	body  string
	base  string
	draft bool
}

// NewPullRequest creates the options for a pull request with the given title.
func NewPullRequest(title string) pullrequestopts {
	return pullrequestopts{title: title}
}

// Body sets the description of the pull request.
func (t pullrequestopts) Body(v string) pullrequestopts {
	t.body = v
	return t
}

// Base sets the branch the pull request merges into. defaults to the default branch of the repository.
func (t pullrequestopts) Base(v string) pullrequestopts {
	t.base = v
	return t
}

// Draft opens the pull request as a draft.
func (t pullrequestopts) Draft() pullrequestopts {
	t.draft = true
	return t
}

func mergepullrequestopts(options ...pullrequestopts) (c pullrequestopts) {
	for _, o := range options {
		if o.title != "" {
			c.title = o.title
		}

		if o.body != "" {
			c.body = o.body
		}

		if o.base != "" {
			c.base = o.base
		}

		c.draft = c.draft || o.draft
	}

	return c
}

// PullRequest opens a pull request from the branch checked out in the change's worktree, see eggit.Change.
// the branch must be pushed first, see eggit.Push. nothing is done when there is no branch, i.e.) there was nothing
// to commit, or when the branch already has an open pull request.
// for local environments `eg compute local` auto-detects a GitHub token via `gh auth token` when the
// repository's remote is github.com and the gh cli is installed. override it explicitly with
// e.g.) eg compute local -e GH_TOKEN=<token>, needed for environments without gh.
func PullRequest(c eggit.Changer, options ...pullrequestopts) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		cfg := mergepullrequestopts(options...)
		if strings.TrimSpace(cfg.title) == "" {
			return fmt.Errorf("unable to open the pull request title not specified: %s", cfg.title)
		}

		// the title, body and base are passed by the environment, they are free text and need no quoting.
		runtime := c.Runtime().
			Environ(_eg.EnvGithubToken, ffigit.Bearer()).
			Environ("EG_PULLREQUEST_TITLE", cfg.title).
			Environ("EG_PULLREQUEST_BODY", cfg.body).
			Environ("EG_PULLREQUEST_BASE", cfg.base)

		return shell.Run(ctx, runtime.New(pullrequestscript(cfg)))
	}
}

// pullrequestscript is the script opening the pull request of the current branch.
func pullrequestscript(cfg pullrequestopts) string {
	create := `gh pr create --head "$branch" --title "$EG_PULLREQUEST_TITLE" --body "$EG_PULLREQUEST_BODY"`
	if cfg.base != "" {
		create += ` --base "$EG_PULLREQUEST_BASE"`
	}

	if cfg.draft {
		create += " --draft"
	}

	// a detached HEAD means there was nothing to commit.
	return strings.Join([]string{
		"set -e",
		`branch=$(git rev-parse --abbrev-ref HEAD)`,
		`if [ "$branch" = "HEAD" ]; then echo "no changes to open a pull request for"; exit 0; fi`,
		`if [ "$(gh pr list --head "$branch" --state open --json number --jq length)" != "0" ]; then echo "pull request already open for $branch"; exit 0; fi`,
		create,
	}, "\n")
}
