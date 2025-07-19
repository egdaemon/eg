package egtarball_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/stretchr/testify/assert"
)

func TestPatternEnv(t *testing.T) {
	t.Run("simple environment expansion", func(t *testing.T) {
		assert.Equal(t, "hello.world", egtarball.EnvPattern("hello.%%env.FOO%%", envx.NewEnvironFromStrings("FOO=world").Map))
		assert.Equal(t, "hello.%env.FOO%", egtarball.EnvPattern("hello.%env.FOO%", envx.NewEnvironFromStrings("FOO=world").Map))
		assert.Equal(t, "hello.", egtarball.EnvPattern("hello.%%env.BAR%%", envx.NewEnvironFromStrings("FOO=world").Map))
	})
}
