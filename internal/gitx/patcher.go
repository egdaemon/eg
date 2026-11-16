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
	w      io.Writer
	clone  string
	repo   string
	branch string
	dry    bool
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
			if out, err := execx.RunAs(ctx, RepositoryUsername(t.clone), "git", "-C", t.clone, "format-patch", "--stdout", "--binary", CloneBase(ctx, t.clone)+".."+head).Output(); err != nil {
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

	base := CloneBase(ctx, t.clone)
	if base == "" {
		debugx.Fn(func() { log.Println("autopatch: carried=none, clone has no recorded base commit") })
		return ""
	}

	username := RepositoryUsername(t.clone)
	out, err := execx.RunAs(ctx, username, "git", "-C", t.clone, "rev-parse", "--verify", "--quiet", "HEAD^{commit}").Output()
	if err != nil {
		debugx.Fn(func() { log.Println("autopatch: carried=none, unable to resolve clone HEAD", err) })
		return ""
	}

	head := strings.TrimSpace(string(out))
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

// commit commits the patch to the branch on top of head, the commits made within the clone, when there are any.
func (t autopatcher) commit(ctx context.Context, head string, patch []byte) error {
	var (
		branch   = t.branchname()
		ref      = "refs/heads/" + branch
		username = RepositoryUsername(t.repo)
	)

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
	if current, err := git(nil, nil, "symbolic-ref", "-q", "HEAD"); err == nil && current == ref {
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

	// the worktree was not updated, make sure the user knows where their changes went.
	branch := strings.TrimPrefix(ref, "refs/heads/")
	log.Printf("autopatch: the changes were committed to branch %q, your worktree was not modified. use `git checkout %s` or `git merge %s` to get them\n", branch, branch, branch)
	return nil
}
