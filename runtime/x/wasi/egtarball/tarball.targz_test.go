package egtarball_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/stretchr/testify/assert"
)

func TestTargz(t *testing.T) {
	assert.Equal(t, "myarchive.tar.gz", egtarball.Targz("myarchive"))
	assert.Equal(t, "path/to/archive.tar.gz", egtarball.Targz("path/to/archive"))
	assert.Equal(t, ".tar.gz", egtarball.Targz(""))
}
