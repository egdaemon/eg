package gitx

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/egdaemon/eg/internal/uuidx"
)

// Checkpoint records the progress of the clone at dir without touching its branches, index or files:
// RefCurrent is moved to the clone's HEAD, and when the working tree differs from what was last saved, i.e.
// from the last checkpoint or else from HEAD, a commit of it on top of HEAD is made at RefCheckpoint.
// it reports whether a checkpoint was made.
func Checkpoint(ctx context.Context, dir string) (made bool, err error) {
	username := RepositoryUsername(dir)

	git := func(env []string, args ...string) (string, error) {
		args = append([]string{"-C", dir}, args...)
		cmd := execx.RunAs(ctx, username, "git", args...)
		if len(env) > 0 {
			// env is passed through the command so it survives running as another user.
			cmd = execx.RunAs(ctx, username, "env", append(append([]string{}, env...), "git")...)
			cmd.Args = append(cmd.Args, args...)
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", errorsx.Wrapf(fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err), "git %s failed", strings.Join(args, " "))
		}

		return strings.TrimSpace(string(out)), nil
	}

	head, err := git(nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return false, err
	}

	if _, err = git(nil, "update-ref", RefCurrent, head); err != nil {
		return false, err
	}

	// the temporary index lives in the repository so it is accessible to the repository's user.
	gitdir, err := git(nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false, err
	}

	index := filepath.Join(gitdir, "eg-checkpoint-"+uuidx.NewV4().String())
	env := []string{"GIT_INDEX_FILE=" + index}
	defer os.Remove(index)

	if _, err = git(env, "read-tree", head); err != nil {
		return false, err
	}

	if _, err = git(env, "add", "-A", "--", "."); err != nil {
		return false, err
	}

	tree, err := git(env, "write-tree")
	if err != nil {
		return false, err
	}

	saved := head
	if last, lerr := git(nil, "rev-parse", "--verify", "--quiet", RefCheckpoint+"^{commit}"); lerr == nil {
		// a checkpoint from before work was committed on top of it is out of date.
		if err := execx.RunAs(ctx, username, "git", "-C", dir, "merge-base", "--is-ancestor", head, last).Run(); err == nil {
			saved = last
		}
	}

	savedtree, err := git(nil, "rev-parse", "--verify", saved+"^{tree}")
	if err != nil {
		return false, err
	}

	if tree == savedtree {
		return false, nil
	}

	// identity is only given when git has none, the user's own is otherwise kept.
	identity := []string{}
	if _, err := git(nil, "config", "user.email"); err != nil {
		identity = []string{"-c", "user.name=eg", "-c", "user.email=eg@localhost"}
	}

	commit, err := git(nil, append(identity, "commit-tree", tree, "-p", head, "-m", "eg: checkpoint")...)
	if err != nil {
		return false, err
	}

	if _, err = git(nil, "update-ref", RefCheckpoint, commit); err != nil {
		return false, err
	}

	debugx.Println("checkpoint", commit, "on", head)
	return true, nil
}

// CheckpointLoop makes a Checkpoint of the clone at dir when started and then every interval until stopped or ctx is
// done. an interval that is not positive makes no checkpoints. a checkpoint that fails is logged and the loop goes on.
// stop ends the loop and waits for a checkpoint in progress to finish, so it is safe to read the clone afterwards.
func CheckpointLoop(ctx context.Context, dir string, interval time.Duration) (stop func()) {
	if interval <= 0 {
		return func() {}
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)

		// stopping is how the loop ends, it is not worth a line in the log.
		errorsx.Log(errorsx.Ignore(timex.NowAndEvery(ctx, interval, func(ctx context.Context) error {
			if _, err := Checkpoint(ctx, dir); err != nil && ctx.Err() == nil {
				log.Println("unable to checkpoint", err)
			}

			return nil
		}), context.Canceled))
	}()

	return func() {
		cancel()
		<-done
	}
}
