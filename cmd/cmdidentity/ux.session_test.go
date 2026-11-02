package cmdidentity_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/stretchr/testify/require"
)

func TestUXSessionFull(t *testing.T) {
	current := &authn.Authn{
		Profile: &authn.Profile{Id: "p1", Display: "alice", Email: "alice@example.com"},
		Account: &authn.Account{Id: "a1", Description: "acme"},
	}
	ident := &authn.Identity{Uid: "u1", Display: "alice", Email: "alice@example.com"}

	require.Equal(t, "identity: u1 - alice - alice@example.com\naccount: a1 - acme\nprofile: p1 - alice - alice@example.com", fmt.Sprintf("%v", cmdidentity.UXSessionFull(ident, current)))
	require.Equal(t, "identity: \naccount: \nprofile: ", fmt.Sprintf("%v", cmdidentity.UXSessionFull(nil, nil)))
}
