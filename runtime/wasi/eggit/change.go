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

type change struct {
	dir     string
	base    string
	runtime shell.Command
}

// Change sets up a git worktree at the given path, thereby allowing multiple concurrent changes to occur.
// an absolute path is used as is, a relative path is relative to the workspace directory, see egenv.WorkspaceDirectory.
// the worktree is created from the current HEAD, see change.Base, the first time it is needed and reused afterwards.
// the worktree's registration and branches live in the repository, so it should be in a directory that outlives the
// module, such as the workspace, instead of a temporary directory which is discarded with the module's container.
func Change(path string) change {
	dir := path
	if !filepath.IsAbs(dir) {
		dir = egenv.WorkspaceDirectory(path)
	}

	return change{dir: dir, runtime: shell.Runtime()}
}

// Base sets the ref the worktree is created from. defaults to the current HEAD.
func (t change) Base(ref string) change {
	t.base = ref
	return t
}

// Changer provides access to a change's worktree.
type Changer interface {
	// Path is the directory of the worktree.
	Path() string
	// Runtime is a shell command running within the worktree.
	Runtime() shell.Command
}

// Path is the directory of the worktree.
func (t change) Path() string {
	return t.dir
}

// Runtime is a shell command running within the worktree.
func (t change) Runtime() shell.Command {
	return t.runtime.Directory(t.dir)
}

// Op ensures the worktree exists and then runs the op returned by fn with the change.
func (t change) Op(fn func(Changer) eg.OpFn) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		if err := t.ensure(ctx); err != nil {
			return err
		}

		return fn(t)(ctx, op)
	}
}

func (t change) ensure(ctx context.Context) error {
	if strings.TrimSpace(t.dir) == "" {
		return fmt.Errorf("unable to setup change path not specified: %s", t.dir)
	}

	script := strings.Join([]string{
		"set -e",
		fmt.Sprintf("if [ ! -e %s ]; then", shquote(filepath.Join(t.dir, ".git"))),
		fmt.Sprintf("mkdir -p %s", shquote(filepath.Dir(t.dir))),
		// the worktree directory may be gone while the repository still has it registered, e.g.) an ephemeral directory.
		"git worktree prune",
		strings.TrimSpace(fmt.Sprintf("git worktree add --detach %s %s", shquote(t.dir), shquote(t.base))),
		fmt.Sprintf(`echo "$(git -C %s rev-parse HEAD)" > "$(git -C %s rev-parse --absolute-git-dir)/eg-base"`, shquote(t.dir), shquote(t.dir)),
		"fi",
	}, "\n")

	return shell.Run(ctx, t.runtime.New(script))
}

type commitopts struct {
	message string
	branch  string
	paths   []string
}

// NewCommit creates the options for a commit with the given message.
func NewCommit(msg string) commitopts {
	return commitopts{message: msg}
}

// Add copies the modified and new files at the paths, relative to the workspace, into the change's worktree
// and includes them in the commit.
// without any paths everything modified within the worktree is committed.
func (t commitopts) Add(paths ...string) commitopts {
	t.paths = append(append([]string(nil), t.paths...), paths...)
	return t
}

// Branch sets the name of the branch the commit is made on. the branch is created when there is something to commit.
func (t commitopts) Branch(name string) commitopts {
	t.branch = name
	return t
}

func mergecommitopts(options ...commitopts) (c commitopts) {
	for _, o := range options {
		if o.message != "" {
			c.message = o.message
		}

		if o.branch != "" {
			c.branch = o.branch
		}

		c.paths = append(c.paths, o.paths...)
	}

	return c
}

// Commit returns an op that commits the changes of the change's worktree to a new branch. does nothing when there are no changes.
// commits are authored by the head commit's author, falling back to egdaemon <noreply@egdaemon.com>.
func Commit(c Changer, options ...commitopts) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return docommit(ctx, c, mergecommitopts(options...))
	}
}

func docommit(ctx context.Context, c Changer, cfg commitopts) error {
	if strings.TrimSpace(cfg.message) == "" {
		return fmt.Errorf("unable to commit message not specified: %s", cfg.message)
	}

	if strings.TrimSpace(cfg.branch) == "" {
		return fmt.Errorf("unable to commit branch not specified: %s", cfg.branch)
	}

	// changes provided by Change.Op are already setup, otherwise the worktree is created when needed.
	if e, ok := c.(interface{ ensure(context.Context) error }); ok {
		if err := e.ensure(ctx); err != nil {
			return err
		}
	}

	if err := shell.Run(ctx, c.Runtime().Newf("git check-ref-format --branch %s", shquote(cfg.branch))); err != nil {
		return fmt.Errorf("invalid branch name %s: %w", cfg.branch, err)
	}

	script := []string{"set -e"}

	quoted := make([]string, 0, len(cfg.paths))
	for _, p := range cfg.paths {
		quoted = append(quoted, shquote(p))
	}

	workspace := egenv.WorkspaceDirectory()
	if len(quoted) > 0 && filepath.Clean(workspace) != filepath.Clean(c.Path()) {
		// only modified and new files are copied, skipping the worktree itself when it is within the workspace.
		script = append(script,
			fmt.Sprintf("dest=%s", shquote(c.Path())),
			fmt.Sprintf("cd %s", shquote(workspace)),
			fmt.Sprintf(`git ls-files -z --modified --others --exclude-standard -- %s | while IFS= read -r -d '' f; do`, strings.Join(quoted, " ")),
			`case "$dest/" in "$PWD/$f"*) continue ;; esac`,
			`if [ -e "$f" ]; then cp -R --parents -- "$f" "$dest"; fi`,
			"done",
			`cd "$dest"`,
		)
	}

	add := "git add -A"
	if len(quoted) > 0 {
		add = "git add -A -- " + strings.Join(quoted, " ")
	}

	// commands do not expose their output so the clean check lives in the script.
	script = append(script,
		add,
		`if git diff --cached --quiet; then echo "no changes to commit"; exit 0; fi`,
		fmt.Sprintf(`if [ "$(git rev-parse --abbrev-ref HEAD)" != %s ]; then git checkout -b %s; fi`, shquote(cfg.branch), shquote(cfg.branch)),
		fmt.Sprintf("git commit -m %s", shquote(cfg.message)),
	)

	return shell.Run(ctx, commitidentity(c.Runtime(), EnvCommit().Committer).New(strings.Join(script, "\n")))
}

type pushconfig struct {
	remote string
}

type pushopts func(*pushconfig)

// PushRemote sets the remote the branch is pushed to.
// defaults to EG_GIT_HEAD_REMOTE and then origin.
func PushRemote(name string) pushopts {
	return func(c *pushconfig) {
		c.remote = name
	}
}

// Push returns an op that pushes the branch committed by Commit to the remote. does nothing when nothing was committed.
// when cloning is disabled (EG_UNSAFE_GIT_CLONE_ENABLED=false), e.g.) eg compute local, there is nothing
// to push to, so the commits are written as a patch to the workspace directory instead, see egenv.WorkspaceDirectory.
// the patch includes the commit message and author, apply it with git am. the token used to push is read
// the same way as ffigit.Bearer and is only provided to git through the environment.
func Push(c Changer, options ...pushopts) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return dopush(ctx, c, options...)
	}
}

func dopush(ctx context.Context, c Changer, options ...pushopts) error {
	cfg := pushconfig{remote: env.String("origin", "EG_GIT_HEAD_REMOTE")}
	for _, opt := range options {
		opt(&cfg)
	}

	if strings.TrimSpace(cfg.remote) == "" {
		return fmt.Errorf("unable to push remote not specified: %s", cfg.remote)
	}

	// a detached HEAD means Commit had nothing to commit.
	script := []string{
		"set -e",
		`branch=$(git rev-parse --abbrev-ref HEAD)`,
		`if [ "$branch" = "HEAD" ]; then echo "no changes to submit"; exit 0; fi`,
	}

	runtime := c.Runtime()

	// without a clone the workload runs against the local checkout, where there is nothing to push to,
	// so the commits are written as a patch for the runner to hand back.
	if !env.Boolean(true, _eg.EnvUnsafeGitCloneEnabled) {
		dir := egenv.WorkspaceDirectory(".egpatch")
		script = append(script,
			`base=$(cat "$(git rev-parse --absolute-git-dir)/eg-base" 2>/dev/null || git rev-parse HEAD~1)`,
			fmt.Sprintf(`dest=%s/"$(echo "$branch" | tr / -)".patch`, shquote(dir)),
			fmt.Sprintf("mkdir -p %s", shquote(dir)),
			`git format-patch --stdout --binary "$base..HEAD" > "$dest"`,
			`echo "patch written to $dest"`,
		)

		return shell.Run(ctx, runtime.New(strings.Join(script, "\n")))
	}

	// the token authenticates over http(s), so with one the push goes to the http(s) form of the remote.
	token := ffigit.Bearer()
	pushcmd := fmt.Sprintf(`git push %s "HEAD:refs/heads/$branch"`, shquote(cfg.remote))
	if strings.TrimSpace(token) != "" {
		pushcmd = httpsremote(cfg.remote) + "\n" + `git push "$target" "HEAD:refs/heads/$branch"`
	}

	script = append(script, pushcmd)

	return shell.Run(ctx, pushauth(runtime, token).New(strings.Join(script, "\n")))
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
