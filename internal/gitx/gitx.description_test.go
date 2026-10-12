package gitx_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/gitx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestDescription(t *testing.T) {
	const pattern = "Pushed: %git.commit.author% - %git.ref%"

	repo, err := git.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	commit := func(name, email string) string {
		h, err := wt.Commit("msg", &git.CommitOptions{
			AllowEmptyCommits: true,
			Author:            &object.Signature{Name: name, Email: email, When: time.Now()},
			Committer:         &object.Signature{Name: name, Email: email, When: time.Now()},
		})
		require.NoError(t, err)
		return h.String()
	}

	t.Run("falls back to the environment when the reference can't be resolved", func(t *testing.T) {
		t.Setenv("USER", "someone")
		require.Equal(t, "Pushed: someone - HEAD", gitx.Description(pattern, repo, "HEAD"))
		require.Equal(t, "[]", gitx.Description("[%git.hash.short%%git.commit.email%]", repo, "HEAD"))
	})

	t.Run("uses the committer name, then email", func(t *testing.T) {
		commit("Jane", "jane@example.com")
		require.Equal(t, "Pushed: Jane - HEAD", gitx.Description(pattern, repo, "HEAD"))

		commit("", "jane@example.com")
		require.Equal(t, "Pushed: jane@example.com - HEAD", gitx.Description(pattern, repo, "HEAD"))
	})

	t.Run("callers position the replacements", func(t *testing.T) {
		h := commit("Jane", "jane@example.com")
		require.Equal(t, "HEAD by Jane <jane@example.com>", gitx.Description("%git.ref% by %git.commit.author% <%git.commit.email%>", repo, "HEAD"))
		require.Equal(t, h+" "+h[:7], gitx.Description("%git.hash% %git.hash.short%", repo, "HEAD"))
	})
}
