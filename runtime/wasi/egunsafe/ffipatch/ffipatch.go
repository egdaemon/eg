// Package ffipatch has helpers for producing and submitting patches of a workload's changes.
// it is not part of the stable api.
package ffipatch

import (
	"context"
	"os"
	"strings"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// Patch generates a patch of the current working tree changes, including new files, and writes it to dest.
func Patch(dest string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return shell.Run(
			ctx,
			shell.Runtime().Directory(egenv.WorkingDirectory()).Newf("git add -N . && git diff HEAD > %s", dest),
		)
	}
}

// single-quote s for safe embedding within a shell command.
func shquote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// SubmitReviewRequest applies patch on a new branch and opens a merge/pull
// request carrying description. The branch is always pushed with plain git;
// only "open the review request" is forge specific, and even that prefers a
// plain git mechanism over a CLI tool wherever the forge supports one:
// GitLab is opened via native git push options (no extra binary needed),
// GitHub via the gh CLI (GitHub has no push-option equivalent), and anything
// else degrades to a plain push plus a textual git request-pull summary
// written next to patch.
func SubmitReviewRequest(patch string, description string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		var (
			rt       = shell.Runtime().Directory(egenv.WorkingDirectory())
			branch   = eggit.EnvCommit().StringReplace("autogentest/%git.hash.short%")
			vcs      = eggit.EnvCanonicalURI()
			descfile = egenv.EphemeralDirectory("eg.git.mr.description")
		)

		if err := os.WriteFile(descfile, []byte(description), 0600); err != nil {
			return errorsx.Wrap(err, "unable to write review request description")
		}

		if err := shell.Run(
			ctx,
			rt.Newf("git checkout -b %s", branch),
			rt.Newf("git apply %s", patch),
			rt.New("git add -A"),
			rt.Newf("git commit -F %s", descfile),
		); err != nil {
			return err
		}

		switch {
		case strings.Contains(vcs, "gitlab.com"):
			return shell.Run(ctx, rt.Newf(
				"git push -o merge_request.create -o merge_request.description=%s -o merge_request.remove_source_branch origin %s",
				shquote(description), branch,
			))
		case strings.Contains(vcs, "github.com"):
			return shell.Run(
				ctx,
				rt.Newf("git push -u origin %s", branch),
				rt.Newf("gh pr create --head %s --body-file %s", branch, descfile),
			)
		default:
			return shell.Run(
				ctx,
				rt.Newf("git push -u origin %s", branch),
				rt.Newf("{ cat %s; echo; git request-pull %s origin %s; } > %s.request-pull",
					descfile, env.String("main", _eg.EnvGitBaseRef), branch, patch),
			)
		}
	}
}
