package compute

import (
	"testing"

	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/stretchr/testify/require"
)

// errAmbiguous repeats identity.ErrAmbiguous since identity cannot be imported by the wasm workloads, they must match.
func TestErrAmbiguousMatchesIdentity(t *testing.T) {
	require.ErrorIs(t, errAmbiguous, identity.ErrAmbiguous)
	require.ErrorIs(t, errorsx.Wrapf(errAmbiguous, "identity %s has %d profiles", "id", 2), identity.ErrAmbiguous)
}
