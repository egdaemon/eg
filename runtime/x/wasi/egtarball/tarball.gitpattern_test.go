package egtarball_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/stretchr/testify/assert"
)

func TestGitPattern(t *testing.T) {
	assert.Equal(
		t,
		"myprefix.%git.commit.year%.%git.commit.month%.%git.commit.day%%git.hash.short%",
		egtarball.GitPattern("myprefix"),
	)
	assert.Equal(
		t,
		".%git.commit.year%.%git.commit.month%.%git.commit.day%%git.hash.short%",
		egtarball.GitPattern(""),
	)
}
