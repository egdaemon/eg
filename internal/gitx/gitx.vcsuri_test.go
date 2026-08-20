package gitx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVCSURI(t *testing.T) {
	t.Run("expected result", func(t *testing.T) {
		assert.Equal(t, "git@github.com:user/repo.git", vcsuri("https://github.com/user/repo.git"), "https GitHub URL")
		assert.Equal(t, "git@gitlab.com:org/sub/repo.git", vcsuri("https://gitlab.com/org/sub/repo.git"), "https GitLab URL")
		assert.Equal(t, "git@git.company.com:8443:org/repo.git", vcsuri("https://git.company.com:8443/org/repo.git"), "HTTPS with port")
		assert.Equal(t, "git@github.com:user/repo.git", vcsuri("ssh://git@github.com/user/repo.git"), "ssh:// normalization")
		assert.Equal(t, "git@gitlab.com:org/repo.git", vcsuri("ssh://gitlab-user@gitlab.com/org/repo.git"), "ssh:// with different user")
		assert.Equal(t, "git@github.com:user/repo.git/", vcsuri("https://github.com/user/repo.git/"), "trailing slash preserved")
		assert.Equal(t, "git@github.com:user/repo.git?param=value", vcsuri("https://github.com/user/repo.git?param=value"), "query parameters preserved")
		assert.Equal(t, "git@gitlab.com:org/sub/sub2/repo.git", vcsuri("https://gitlab.com/org/sub/sub2/repo.git"), "deep subgroup path")
	})
}
