package egtarball_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	testx.Tempenvvar(_eg.EnvGitHeadCommit, "aabbccdd11223344aabbccdd11223344aabbccdd", func() {
		testx.Tempenvvar(_eg.EnvGitHeadCommitTimestamp, "2024-03-15T10:00:00Z", func() {
			assert.Equal(t, "myarchive.2024.3.15aabbccd", egtarball.Name("myarchive.%git.commit.year%.%git.commit.month%.%git.commit.day%%git.hash.short%"))
		})
	})

	t.Run("no substitutions", func(t *testing.T) {
		assert.Equal(t, "literal.tar.gz", egtarball.Name("literal.tar.gz"))
	})
}
