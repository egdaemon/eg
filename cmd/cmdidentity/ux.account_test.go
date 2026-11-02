package cmdidentity_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/stretchr/testify/require"
)

func TestUXAccountFull(t *testing.T) {
	require.Equal(t, "a1 - acme", fmt.Sprintf("%v", cmdidentity.UXAccountFull(&authn.Account{Id: "a1", Description: "acme"})))
	require.Equal(t, "a1", fmt.Sprintf("%v", cmdidentity.UXAccountFull(&authn.Account{Id: "a1"})))
	require.Equal(t, "a1 - acme  |", fmt.Sprintf("%-11v|", cmdidentity.UXAccountFull(&authn.Account{Id: "a1", Description: "acme"})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.UXAccountFull(nil)))
}
