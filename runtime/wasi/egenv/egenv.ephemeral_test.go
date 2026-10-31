package egenv_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestPathRelativeEphemeral(t *testing.T) {
	t.Setenv("TMPDIR", "/tmp-eph")

	require.Equal(t, "a/b.go", egenv.PathRelativeEphemeral("/tmp-eph/a/b.go"))
	require.Equal(t, "/elsewhere/b.go", egenv.PathRelativeEphemeral("/elsewhere/b.go"))
	require.Equal(t, "/tmp-eph-other/b.go", egenv.PathRelativeEphemeral("/tmp-eph-other/b.go"))
	require.Equal(t, "example.com/mod/b.go", egenv.PathRelativeEphemeral("example.com/mod/b.go"))
}
