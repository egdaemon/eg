package compute

import (
	"testing"

	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestDefaultBranch(t *testing.T) {
	id := uuidx.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")

	t.Run("is the user, the workload and the end of the run id", func(t *testing.T) {
		require.Regexp(t, `^[A-Za-z0-9._-]+/eg-build-099a8057$`, defaultbranch("build", id))
	})

	t.Run("the default workload has no name to include", func(t *testing.T) {
		require.Regexp(t, `^[A-Za-z0-9._-]+/eg-099a8057$`, defaultbranch("", id))
		require.Regexp(t, `^[A-Za-z0-9._-]+/eg-099a8057$`, defaultbranch("  ", id))
	})

	t.Run("replaces characters that are not valid in a branch name", func(t *testing.T) {
		require.Regexp(t, `^[A-Za-z0-9._-]+/eg-my-workload-099a8057$`, defaultbranch("my workload", id))
		require.Regexp(t, `^[A-Za-z0-9._-]+/eg-a-b-099a8057$`, defaultbranch("a/b", id))
	})
}
