package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/uuidx"
)

// PatchError is returned when a patch could not be applied, it carries the patch so the changes are not lost.
type PatchError struct {
	Patch []byte
	Err   error
}

func (t *PatchError) Error() string {
	return fmt.Sprintf("unable to apply changes, the patch is attached to the error: %v", t.Err)
}

func (t *PatchError) Unwrap() error {
	return t.Err
}

// WriteTo writes the patch that could not be applied.
func (t *PatchError) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(t.Patch)
	return int64(n), err
}

// PatchWrite writes the patch carried by err, if any, to fallback so the changes are not lost.
// err is returned unchanged.
func PatchWrite(err error, fallback io.Writer) error {
	var perr *PatchError
	if errors.As(err, &perr) {
		if _, werr := perr.WriteTo(fallback); werr != nil {
			return errors.Join(err, werr)
		}
	}

	return err
}

type patchconfig struct {
	branch string
}

type patchoption = func(*patchconfig)

type patcher struct {
	dir  string
	opts []patchoption
}

// Patch creates a patcher for the worktree at dir.
func Patch(dir string) patcher {
	return patcher{dir: dir}
}

// Branch sets the destination branch, the changes are committed to it regardless of which branch
// is checked out. the branch is created from the current head when it does not exist. when no branch
// is set the changes are applied to the worktree without committing.
func (t patcher) Branch(name string) patcher {
	t.opts = append(t.opts[:len(t.opts):len(t.opts)], func(c *patchconfig) {
		c.branch = name
	})
	return t
}

// Apply applies the patch to the worktree, changes that are already present are skipped. if the
// patch cannot be applied the worktree is left untouched and a *PatchError carrying the patch is returned.
func (t patcher) Apply(ctx context.Context, patch []byte) error {
	if len(patch) == 0 {
		return nil
	}

	var (
		cfg      = langx.Clone(patchconfig{}, t.opts...)
		username = RepositoryUsername(t.dir)
	)

	git := func(stdin []byte, args ...string) (string, error) {
		cmd := execx.RunAs(ctx, username, "git", append([]string{"-C", t.dir}, args...)...)
		if stdin != nil {
			cmd.Stdin = bytes.NewReader(stdin)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "git %s failed", strings.Join(args, " "))
		}
		return strings.TrimSpace(string(out)), nil
	}

	fail := func(err error) error {
		return &PatchError{Patch: patch, Err: err}
	}

	if cfg.branch != "" {
		if current, err := git(nil, "rev-parse", "--abbrev-ref", "HEAD"); err != nil {
			return fail(err)
		} else if current != cfg.branch {
			if _, err := git(nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+cfg.branch); err == nil {
				_, err = git(nil, "checkout", "-q", cfg.branch)
				if err != nil {
					return fail(err)
				}
			} else if _, err = git(nil, "checkout", "-q", "-b", cfg.branch); err != nil {
				return fail(err)
			}
		}
	}

	// already present, e.g.) the changes were applied by whatever made them.
	if _, err := git(patch, "apply", "--reverse", "--check", "-"); err == nil {
		return nil
	}

	if _, err := git(patch, "apply", "--check", "-"); err != nil {
		return fail(err)
	}

	// when committing only the patch's own changes are staged, leaving unrelated untracked files alone.
	applyargs := []string{"apply", "--binary", "-"}
	if cfg.branch != "" {
		applyargs = []string{"apply", "--binary", "--index", "-"}
	}

	if _, err := git(patch, applyargs...); err != nil {
		return fail(err)
	}

	if cfg.branch == "" {
		return nil
	}

	if _, err := git(nil, "commit", "-q", "-m", "eg: apply workload changes"); err != nil {
		return fail(err)
	}

	return nil
}

type autopatcher struct {
	w        io.Writer
	clone    string
	repo     string
	branch   string
	session  string
	nobranch bool
	dry      bool
}

// AutoPatch creates a patcher which collects the changes made within a clone and commits them to a
// new branch of the repository it was cloned from, the repository's worktree is never touched.
// w receives the patch when it cannot be committed, or when DryRun is enabled, e.g.) os.Stdout or io.Discard.
func AutoPatch(w io.Writer) autopatcher {
	return autopatcher{w: w}
}

// From sets the clone the changes are collected from.
func (t autopatcher) From(clonedir string) autopatcher {
	t.clone = clonedir
	return t
}

// To sets the repository the branch is created in.
func (t autopatcher) To(repodir string) autopatcher {
	t.repo = repodir
	return t
}

// Branch sets the name of the branch the changes are committed to, the first name that is not blank is used and
// when there is none it defaults to eg/{uuidv4}. if the branch exists the changes are committed on top of it,
// otherwise it is created from the current head.
func (t autopatcher) Branch(names ...string) autopatcher {
	t.branch = stringsx.FirstNonBlank(names...)
	return t
}

// Session commits the changes to the ref of the session, see SessionRef, instead of to the branch: the work
// does not depend on what the branch is called, which can change while the run is going. the commits are
// only extended. when Branch is also given, and AutoBranch is not disabled, the branch is then created or fast
// forwarded to the ref, the same as a commit straight to the branch would have left it.
func (t autopatcher) Session(id string) autopatcher {
	t.session = id
	return t
}

// AutoBranch whether the branch is created from the session ref once the changes are committed, it defaults to
// enabled. it only applies with Session, a commit straight to a branch always creates it. disabled, the ref is
// all that is written and whatever manages the branch takes it from there.
func (t autopatcher) AutoBranch(b bool) autopatcher {
	t.nobranch = !b
	return t
}

// DryRun when enabled the changes are written as a patch instead of being committed.
func (t autopatcher) DryRun(b bool) autopatcher {
	t.dry = b
	return t
}

func (t autopatcher) branchname() string {
	if t.branch != "" {
		return t.branch
	}

	return "eg/" + uuidx.NewV4().String()
}

// Apply collects the changes from the clone and commits them to the branch. the repository's worktree,
// index and head are left untouched. if the changes cannot be committed the patch is written so they are not lost.
//
// both the commits made within the clone, since the commit it started at (see CloneBase), and the uncommitted
// changes are collected. the commits are carried over as they are, keeping their authors and messages, and the
// uncommitted changes are committed on top of them.
func (t autopatcher) Apply(ctx context.Context) error {
	patch, err := Changes(ctx, t.clone, t.repo)
	if err != nil {
		return err
	}

	head := t.carried(ctx)
	debugx.Fn(func() {
		log.Printf("autopatch: clone=%q repo=%q branch=%q dry=%t patch=%d bytes carried=%q\n", t.clone, t.repo, t.branch, t.dry, len(patch), head)
	})

	if len(patch) == 0 && head == "" {
		debugx.Fn(func() { log.Println("autopatch: path=noop, no changes and no carried commits") })
		return nil
	}

	if t.dry {
		debugx.Fn(func() { log.Println("autopatch: path=dry-run, writing patch instead of committing") })
		if head != "" {
			if out, err := execx.RunAs(ctx, RepositoryUsername(t.clone), "git", "-C", t.clone, "format-patch", "--stdout", "--binary", t.base(ctx, head)+".."+head).Output(); err != nil {
				return errorsx.Wrapf(err, "unable to format the commits: %s", t.clone)
			} else if _, err = t.w.Write(out); err != nil {
				return err
			}
		}

		_, err = t.w.Write(patch)
		return err
	}

	debugx.Fn(func() { log.Println("autopatch: path=commit, committing to branch") })
	if err = t.commit(ctx, head, patch); err != nil {
		debugx.Fn(func() { log.Println("autopatch: commit failed, writing patch to fallback writer", err) })
		return PatchWrite(&PatchError{Patch: patch, Err: err}, t.w)
	}

	return nil
}

// carried returns the commit the clone is at when commits were made within it since it started, blank otherwise.
func (t autopatcher) carried(ctx context.Context) string {
	if !isSeparateClone(t.clone, t.repo) {
		debugx.Fn(func() { log.Println("autopatch: carried=none, clone is not separate from the repository") })
		return ""
	}

	username := RepositoryUsername(t.clone)
	out, err := execx.RunAs(ctx, username, "git", "-C", t.clone, "rev-parse", "--verify", "--quiet", "HEAD^{commit}").Output()
	if err != nil {
		debugx.Fn(func() { log.Println("autopatch: carried=none, unable to resolve clone HEAD", err) })
		return ""
	}

	head := strings.TrimSpace(string(out))
	base := t.base(ctx, head)
	if base == "" {
		debugx.Fn(func() { log.Println("autopatch: carried=none, clone has no recorded base commit") })
		return ""
	}

	if head == base {
		debugx.Fn(func() { log.Println("autopatch: carried=none, clone HEAD is at its base", base) })
		return ""
	}

	// a clone that was reset or rebased away from its base has no commits to carry.
	if err := execx.RunAs(ctx, username, "git", "-C", t.clone, "merge-base", "--is-ancestor", base, head).Run(); err != nil {
		debugx.Fn(func() { log.Println("autopatch: carried=none, base is not an ancestor of HEAD", base, head) })
		return ""
	}

	debugx.Fn(func() { log.Println("autopatch: carried=commits", base, "..", head) })
	return head
}

// base is the commit the commits of the clone are counted from: the latest commit of the user's workspace the clone
// knows of when head is built on it, i.e. after the clone was rebased onto it, otherwise the commit the clone started at.
func (t autopatcher) base(ctx context.Context, head string) string {
	base := CloneBase(ctx, t.clone)
	username := RepositoryUsername(t.clone)

	out, err := execx.RunAs(ctx, username, "git", "-C", t.clone, "rev-parse", "--verify", "--quiet", RefLatest+"^{commit}").Output()
	if err != nil {
		return base
	}

	latest := strings.TrimSpace(string(out))
	if latest == "" || latest == base {
		return base
	}

	if err := execx.RunAs(ctx, username, "git", "-C", t.clone, "merge-base", "--is-ancestor", latest, head).Run(); err != nil {
		return base
	}

	return latest
}

// target is the ref the changes are committed to.
func (t autopatcher) target() string {
	if t.session != "" {
		return SessionRef(t.session)
	}

	return "refs/heads/" + t.branchname()
}

// commit commits the patch to the target on top of head, the commits made within the clone, when there are any.
func (t autopatcher) commit(ctx context.Context, head string, patch []byte) error {
	if err := t.commitref(ctx, head, patch); err != nil {
		return err
	}

	if t.session == "" {
		return nil
	}

	return t.materialize(ctx)
}

// repogit runs git in the repository the changes are committed to, as its user.
func (t autopatcher) repogit(ctx context.Context) func(env []string, stdin []byte, args ...string) (string, error) {
	username := RepositoryUsername(t.repo)

	git := func(env []string, stdin []byte, args ...string) (string, error) {
		args = append([]string{"-C", t.repo}, args...)
		var cmd *exec.Cmd
		if len(env) > 0 {
			// env is passed through the command so it survives running as another user.
			cmd = execx.RunAs(ctx, username, "env", append(append([]string{}, env...), "git")...)
			cmd.Args = append(cmd.Args, args...)
		} else {
			cmd = execx.RunAs(ctx, username, "git", args...)
		}
		if stdin != nil {
			cmd.Stdin = bytes.NewReader(stdin)
		}
		out, err := cmd.Output()
		if err != nil {
			var stderr []byte
			if eerr, ok := err.(*exec.ExitError); ok {
				stderr = eerr.Stderr
			}
			return "", errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(stderr)), err), "git %s failed", strings.Join(args, " "))
		}
		return strings.TrimSpace(string(out)), nil
	}
	return git
}

// commitref commits to the target, see commit.
func (t autopatcher) commitref(ctx context.Context, head string, patch []byte) error {
	var (
		ref    = t.target()
		branch = strings.TrimPrefix(ref, "refs/heads/")
		git    = t.repogit(ctx)
	)

	parent, err := git(nil, nil, "rev-parse", "--verify", "--quiet", ref)
	oldtip := parent
	if err != nil {
		oldtip = ""
		if parent, err = git(nil, nil, "rev-parse", "--verify", "HEAD"); err != nil {
			return err
		}
		debugx.Fn(func() { log.Println("autopatch: branch does not exist, creating from HEAD", branch, parent) })
	} else {
		debugx.Fn(func() { log.Println("autopatch: branch exists, extending", branch, oldtip) })
	}

	if head != "" {
		// bring the commits, and their history, into the repository.
		abs, err := filepath.Abs(t.clone)
		if err != nil {
			return err
		}

		if _, err = git(nil, nil, "fetch", "-q", "--no-tags", "file://"+abs, "HEAD"); err != nil {
			return err
		}

		// an existing branch is only extended, anything else would rewrite what it already has.
		if oldtip != "" {
			if _, err = git(nil, nil, "merge-base", "--is-ancestor", oldtip, head); err != nil {
				return errorsx.Wrapf(err, "the commits do not extend %s", branch)
			}
		}

		parent = head
		debugx.Fn(func() { log.Println("autopatch: fetched clone commits, parent is now", parent) })
	}

	if len(patch) == 0 {
		debugx.Fn(func() { log.Println("autopatch: no uncommitted changes, only updating ref", ref, parent) })
		return t.updateref(git, ref, parent, oldtip)
	}

	// the temporary index lives in the repository so it is accessible to the repository's user.
	gitdir, err := git(nil, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}

	indexenv := []string{"GIT_INDEX_FILE=" + filepath.Join(gitdir, "eg-autopatch-"+uuidx.NewV4().String())}
	defer os.Remove(strings.TrimPrefix(indexenv[0], "GIT_INDEX_FILE="))

	if _, err = git(indexenv, nil, "read-tree", parent); err != nil {
		return err
	}

	if _, err = git(indexenv, patch, "apply", "--cached", "--binary", "-"); err != nil {
		return err
	}

	tree, err := git(indexenv, nil, "write-tree")
	if err != nil {
		return err
	}

	commit, err := git(nil, nil, "commit-tree", tree, "-p", parent, "-m", "eg: apply workload changes")
	if err != nil {
		return err
	}

	debugx.Fn(func() { log.Println("autopatch: committed uncommitted changes on top of parent", commit, parent) })
	return t.updateref(git, ref, commit, oldtip)
}

// updateref points ref at commit, when ref is checked out the worktree is updated as well, ref must be at oldtip, or must not exist when oldtip is blank.
func (t autopatcher) updateref(git func([]string, []byte, ...string) (string, error), ref, commit, oldtip string) error {
	// the branch is checked out, fast forward it so the index and worktree receive the changes, local
	// modifications are kept and the merge refuses, leaving the ref untouched, if they overlap with the changes.
	if current, err := git(nil, nil, "symbolic-ref", "-q", "HEAD"); strings.HasPrefix(ref, "refs/heads/") && err == nil && current == ref {
		debugx.Fn(func() { log.Println("autopatch: branch is checked out, fast forwarding the worktree", ref, commit) })
		_, err = git(nil, nil, "merge", "--ff-only", "-q", commit)
		return err
	}

	args := []string{"update-ref", ref, commit}
	if oldtip != "" {
		args = append(args, oldtip)
	} else {
		args = append(args, strings.Repeat("0", len(commit)))
	}

	if _, err := git(nil, nil, args...); err != nil {
		return err
	}

	if !strings.HasPrefix(ref, "refs/heads/") {
		return nil
	}

	// the worktree was not updated, make sure the user knows where their changes went.
	branch := strings.TrimPrefix(ref, "refs/heads/")
	log.Printf("autopatch: the changes were committed to branch %q, your worktree was not modified. use `git checkout %s` or `git merge %s` to get them\n", branch, branch, branch)
	return nil
}

// materialize points the branch at the session ref, creating it or fast forwarding it, once the changes are committed to
// the ref. nothing is done when the branch is not wanted, or has no name, and the ref is where to find the changes. a
// branch that is not an ancestor of the ref is left alone: it was changed by something else.
func (t autopatcher) materialize(ctx context.Context) error {
	ref := SessionRef(t.session)

	if t.nobranch || t.branch == "" {
		log.Printf("autopatch: the changes were committed to %s, use `git branch <name> %s` to get a branch for them\n", ref, ref)
		return nil
	}

	var (
		git       = t.repogit(ctx)
		branchref = "refs/heads/" + t.branch
	)

	tip, err := git(nil, nil, "rev-parse", "--verify", ref)
	if err != nil {
		return err
	}

	oldtip, err := git(nil, nil, "rev-parse", "--verify", "--quiet", branchref)
	if err != nil {
		oldtip = ""
	}

	if oldtip == tip {
		return nil
	}

	if oldtip != "" {
		if _, err = git(nil, nil, "merge-base", "--is-ancestor", oldtip, tip); err != nil {
			return errorsx.Wrapf(err, "%s is not behind %s, the changes are in %s", t.branch, ref, ref)
		}
	}

	return t.updateref(git, branchref, tip, oldtip)
}
