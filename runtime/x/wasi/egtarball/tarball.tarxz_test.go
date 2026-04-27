package egtarball_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/stretchr/testify/assert"
)

func TestTarxz(t *testing.T) {
	assert.Equal(t, "myarchive.tar.xz", egtarball.Tarxz("myarchive"))
	assert.Equal(t, "path/to/archive.tar.xz", egtarball.Tarxz("path/to/archive"))
	assert.Equal(t, ".tar.xz", egtarball.Tarxz(""))
}
