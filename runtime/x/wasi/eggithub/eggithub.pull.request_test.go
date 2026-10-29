package eggithub

import (
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

type changer struct {
	cmd shell.Command
}

func (t changer) Path(...string) string { return "/worktree" }

func (t changer) Runtime() shell.Command { return t.cmd }

func recorded() (changer, *shell.Recorder) {
	cmd := shell.NewLocal()
	rec := shell.NewRecorder(&cmd)
	return changer{cmd: cmd}, rec
}

func TestPullRequest(t *testing.T) {
	t.Setenv(_eg.EnvGithubToken, "token")

	t.Run("opens a pull request for the current branch with the title and body from the environment", func(t *testing.T) {
		c, rec := recorded()

		err := PullRequest(c, NewPullRequest("a title").Body("a body"))(t.Context(), nil)
		require.NoError(t, err)

		require.Len(t, rec.Results(), 1)
		result := rec.Results()[0]
		require.Contains(t, result, "EG_PULLREQUEST_TITLE=a title")
		require.Contains(t, result, "EG_PULLREQUEST_BODY=a body")
		require.Contains(t, result, `gh pr create --head "$branch" --title "$EG_PULLREQUEST_TITLE" --body "$EG_PULLREQUEST_BODY"`)
		require.NotContains(t, result, "--base")
		require.NotContains(t, result, "--draft")
	})

	t.Run("sets the base and draft", func(t *testing.T) {
		c, rec := recorded()

		require.NoError(t, PullRequest(c, NewPullRequest("t").Base("main").Draft())(t.Context(), nil))
		require.Contains(t, rec.Result(), `--base "$EG_PULLREQUEST_BASE" --draft`)
		require.Contains(t, rec.Result(), "EG_PULLREQUEST_BASE=main")
	})

	t.Run("later options override earlier ones", func(t *testing.T) {
		got := mergepullrequestopts(NewPullRequest("a").Body("x"), NewPullRequest("b").Draft(), pullrequestopts{})
		require.Equal(t, pullrequestopts{title: "b", body: "x", draft: true}, got)
	})

	t.Run("requires a title", func(t *testing.T) {
		c, rec := recorded()

		require.Error(t, PullRequest(c, NewPullRequest(" "))(t.Context(), nil))
		require.Empty(t, rec.Results())
	})

	t.Run("does nothing without a branch or when a pull request is open", func(t *testing.T) {
		script := pullrequestscript(NewPullRequest("t"))
		require.True(t, strings.Contains(script, `if [ "$branch" = "HEAD" ]; then`))
		require.Contains(t, script, `gh pr list --head "$branch" --state open --json number --jq length`)
		require.Less(t, strings.Index(script, "gh pr list"), strings.Index(script, "gh pr create"))
	})
}
