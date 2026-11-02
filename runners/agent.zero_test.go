package runners

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZeroAgent(t *testing.T) {
	t.Run("provides the options of the agent options", func(t *testing.T) {
		assert.Equal(t, []string{"--network", "host"}, ZeroAgent(AgentOptionCommandLine("--network", "host")).Options())
	})

	t.Run("options are applied in order", func(t *testing.T) {
		assert.Equal(t, []string{"--a", "--b"}, ZeroAgent(AgentOptionCommandLine("--a"), AgentOptionCommandLine("--b")).Options())
	})

	t.Run("a noop option provides nothing", func(t *testing.T) {
		assert.Empty(t, ZeroAgent(AgentOptionNoop).Options())
	})

	t.Run("without options provides nothing", func(t *testing.T) {
		assert.Empty(t, ZeroAgent().Options())
	})
}
