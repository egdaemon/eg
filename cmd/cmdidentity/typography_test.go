package cmdidentity_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/stretchr/testify/require"
)

func TestTypographyAccountCompact(t *testing.T) {
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	require.Equal(t, id, fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id})))
	require.Equal(t, id, fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Display: "  "})))
	require.Equal(t, "0190a1b2 - acme", fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Display: "acme"})))
	require.Equal(t, "0190a1b2 - aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", fmt.Sprintf("%s", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Display: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})))
	require.Equal(t, "0190a1b2 - aaaaaaaaa", fmt.Sprintf("%.20v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Display: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(nil)))
	require.Equal(t, "0190a1b2 - acme    |", fmt.Sprintf("%-19v|", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Display: "acme"})))
	require.Equal(t, "     0190a1b2 - acme", fmt.Sprintf("%20s", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Display: "acme"})))
}

func TestTypographyAuthn(t *testing.T) {
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	profile := &authn.Profile{Id: "p1", AccountId: id, Display: "alice"}

	require.Equal(t, "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b - alice", fmt.Sprintf("%v", cmdidentity.TypographyAuthn(&authn.Authn{Profile: profile, Account: &authn.Account{Id: id}})))
	require.Equal(t, "0190a1b2 - acme - alice", fmt.Sprintf("%v", cmdidentity.TypographyAuthn(&authn.Authn{Profile: profile, Account: &authn.Account{Id: id, Display: "acme"}})))
	require.Equal(t, "0190a1b2 - acme - alice  |", fmt.Sprintf("%-25v|", cmdidentity.TypographyAuthn(&authn.Authn{Profile: profile, Account: &authn.Account{Id: id, Display: "acme"}})))
}

func TestTypographyProfileCompact(t *testing.T) {
	require.Equal(t, "alice", fmt.Sprintf("%v", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: "alice"})))
	require.Equal(t, "p1", fmt.Sprintf("%s", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: " "})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.TypographyProfileCompact(nil)))
	require.Equal(t, "alice    |", fmt.Sprintf("%-9v|", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: "alice"})))
	require.Equal(t, "    alice", fmt.Sprintf("%9s", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: "alice"})))
}
