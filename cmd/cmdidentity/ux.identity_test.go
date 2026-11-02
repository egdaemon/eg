package cmdidentity_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/stretchr/testify/require"
)

func TestUXIdentityFull(t *testing.T) {
	require.Equal(t, "u1 - alice - alice@example.com", fmt.Sprintf("%v", cmdidentity.UXIdentityFull(&authn.Identity{Id: "i1", Uid: "u1", Display: "alice", Email: "alice@example.com"})))
	require.Equal(t, "u1 - alice@example.com", fmt.Sprintf("%v", cmdidentity.UXIdentityFull(&authn.Identity{Uid: "u1", Display: " ", Email: "alice@example.com"})))
	require.Equal(t, "u1 - alice  |", fmt.Sprintf("%-12v|", cmdidentity.UXIdentityFull(&authn.Identity{Uid: "u1", Display: "alice"})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.UXIdentityFull(nil)))
}
