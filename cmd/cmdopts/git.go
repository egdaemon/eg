package cmdopts

import (
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/go-git/go-git/v6"
)

// VCS the repository a command acts on, embedded by the commands that detect the account from it.
type VCS struct {
	VcsURI string `name:"vcs-uri" help:"canonical uri of the repository, used to detect the account" default:"${vars_vcs_uri}"`
}

// WorkspaceVCSURI the canonical vcs uri of the repository containing the working directory,
// blank when there is none.
func WorkspaceVCSURI() string {
	repo, err := git.PlainOpen(gitx.DetectRoot())
	if err != nil {
		return ""
	}

	// a repository without the default remote is an expected state, not an error.
	uri, err := gitx.CanonicalURI(repo, git.DefaultRemoteName)
	if err != nil {
		return ""
	}

	return uri
}
