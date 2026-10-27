package eggit

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type submitbranchconfig struct {
	remote string
	base   string
}

// SubmitBranchOption configures SubmitBranch.
type SubmitBranchOption func(*submitbranchconfig)

// SubmitBranchRemote sets the remote the branch is pushed to.
// defaults to EG_GIT_HEAD_REMOTE and then origin.
func SubmitBranchRemote(name string) SubmitBranchOption {
	return func(c *submitbranchconfig) {
		c.remote = name
	}
}

// SubmitBranchBase sets the ref the branch is created from.
// defaults to the current HEAD.
func SubmitBranchBase(ref string) SubmitBranchOption {
	return func(c *submitbranchconfig) {
		c.base = ref
	}
}

// SubmitBranch commits the changes in the workspace to a new branch and pushes it.
// when cloning is disabled (EG_UNSAFE_GIT_CLONE_ENABLED=false), e.g.) eg compute local, there is nothing
// to push to, so the changes are written as a patch to the cache directory instead, see egenv.CacheDirectory.
// does nothing when the workspace has no changes. commits are authored by the head commit's author,
// falling back to egdaemon <noreply@egdaemon.com>. the token used to push is read
// the same way as ffigit.Bearer and is only provided to git through the environment.
func SubmitBranch(branch, message string, options ...SubmitBranchOption) eg.OpFn {
	return submitbranch(shell.Runtime(), branch, message, options...)
}

func submitbranch(runtime shell.Command, branch, message string, options ...SubmitBranchOption) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		if strings.TrimSpace(branch) == "" {
			return fmt.Errorf("unable to submit branch name not specified: %s", branch)
		}

		if strings.TrimSpace(message) == "" {
			return fmt.Errorf("unable to submit branch commit message not specified: %s", message)
		}

		cfg := submitbranchconfig{remote: env.String("origin", "EG_GIT_HEAD_REMOTE")}
		for _, opt := range options {
			opt(&cfg)
		}

		if strings.TrimSpace(cfg.remote) == "" {
			return fmt.Errorf("unable to submit branch remote not specified: %s", cfg.remote)
		}

		if err := shell.Run(ctx, runtime.Newf("git check-ref-format --branch %s", shquote(branch))); err != nil {
			return fmt.Errorf("invalid branch name %s: %w", branch, err)
		}

		// without a clone the workload runs against the local checkout, where there is nothing to push to,
		// so the changes are written as a patch for the runner to hand back.
		if !env.Boolean(true, _eg.EnvUnsafeGitCloneEnabled) {
			dest := egenv.CacheDirectory("egpatches", strings.ReplaceAll(branch, "/", "-")+".patch")
			script := strings.Join([]string{
				"set -e",
				`if [ -z "$(git status --porcelain)" ]; then echo "no changes to submit"; exit 0; fi`,
				fmt.Sprintf("mkdir -p %s", shquote(filepath.Dir(dest))),
				"git add -N .",
				fmt.Sprintf("git diff --binary HEAD > %s", shquote(dest)),
				fmt.Sprintf(`echo "patch written to "%s`, shquote(dest)),
			}, "\n")

			return shell.Run(ctx, runtime.New(script))
		}

		// the token authenticates over http(s), so with one the push goes to the http(s) form of the remote.
		token := ffigit.Bearer()
		push := fmt.Sprintf("git push %s %s", shquote(cfg.remote), shquote(branch))
		if strings.TrimSpace(token) != "" {
			push = httpsremote(cfg.remote) + "\n" + fmt.Sprintf(`git push "$target" %s`, shquote(branch))
		}

		// commands do not expose their output so the clean check lives in the script.
		script := strings.Join([]string{
			"set -e",
			`if [ -z "$(git status --porcelain)" ]; then echo "no changes to submit"; exit 0; fi`,
			fmt.Sprintf("git checkout -b %s", strings.TrimSpace(shquote(branch)+" "+shquote(cfg.base))),
			"git add -A",
			fmt.Sprintf("git commit -m %s", shquote(message)),
			push,
		}, "\n")

		return shell.Run(ctx, commitidentity(pushauth(runtime, token), EnvCommit().Committer).New(script))
	}
}

// httpsremote is a script setting $target to the url of the remote, converting ssh urls
// (git@host:org/repo.git and ssh://git@host/org/repo.git) to https. other urls are kept as is.
func httpsremote(remote string) string {
	return strings.Join([]string{
		fmt.Sprintf("target=$(git remote get-url %s)", shquote(remote)),
		`case "$target" in`,
		`http://*|https://*) ;;`,
		`ssh://*@*) target="https://${target#ssh://*@}" ;;`,
		`*@*:*) rest="${target#*@}"; target="https://${rest%%:*}/${rest#*:}" ;;`,
		`esac`,
	}, "\n")
}

const (
	// identity used to commit when the head commit doesn't provide one.
	defaultCommitAuthor = "egdaemon"
	defaultCommitEmail  = "noreply@egdaemon.com"
)

// commit as the author of the head commit, falling back to egdaemon, since the environment
// the commit is made in may not have an identity configured for git.
func commitidentity(runtime shell.Command, who signature) shell.Command {
	name, email := strings.TrimSpace(who.Name), strings.TrimSpace(who.Email)
	if name == "" {
		name = defaultCommitAuthor
	}

	if email == "" {
		email = defaultCommitEmail
	}

	return runtime.
		Environ("GIT_AUTHOR_NAME", name).
		Environ("GIT_AUTHOR_EMAIL", email).
		Environ("GIT_COMMITTER_NAME", name).
		Environ("GIT_COMMITTER_EMAIL", email)
}

// provide the token to git for the lifetime of the command only,
// keeping it out of the remote url, git config and the command text.
func pushauth(runtime shell.Command, token string) shell.Command {
	if strings.TrimSpace(token) == "" {
		return runtime
	}

	creds := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))

	return runtime.
		Environ("GIT_CONFIG_COUNT", 1).
		Environ("GIT_CONFIG_KEY_0", "http.extraheader").
		Environ("GIT_CONFIG_VALUE_0", "Authorization: Basic "+creds)
}

// quote a value for use as a single shell word. an empty value produces nothing
// so optional arguments can be omitted.
func shquote(s string) string {
	if s == "" {
		return ""
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
