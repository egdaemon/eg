package cmdidentity_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
)

func TestTypographyAccountCompact(t *testing.T) {
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	require.Equal(t, id, fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id})))
	require.Equal(t, id, fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Description: "  "})))
	require.Equal(t, "0190a1b2 - acme", fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Description: "acme"})))
	require.Equal(t, "0190a1b2 - aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", fmt.Sprintf("%s", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Description: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})))
	require.Equal(t, "0190a1b2 - aaaaaaaaa", fmt.Sprintf("%.20v", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Description: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(nil)))
	require.Equal(t, "0190a1b2 - acme    |", fmt.Sprintf("%-19v|", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Description: "acme"})))
	require.Equal(t, "     0190a1b2 - acme", fmt.Sprintf("%20s", cmdidentity.TypographyAccountCompact(&authn.Account{Id: id, Description: "acme"})))
}

func TestTypographyAuthn(t *testing.T) {
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	profile := &authn.Profile{Id: "p1", AccountId: id, Display: "alice"}

	require.Equal(t, "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b - alice", fmt.Sprintf("%v", cmdidentity.TypographyAuthn(&authn.Authn{Profile: profile, Account: &authn.Account{Id: id}})))
	require.Equal(t, "0190a1b2 - acme - alice", fmt.Sprintf("%v", cmdidentity.TypographyAuthn(&authn.Authn{Profile: profile, Account: &authn.Account{Id: id, Description: "acme"}})))
	require.Equal(t, "0190a1b2 - acme - alice  |", fmt.Sprintf("%-25v|", cmdidentity.TypographyAuthn(&authn.Authn{Profile: profile, Account: &authn.Account{Id: id, Description: "acme"}})))
}

func TestTypographyProfileCompact(t *testing.T) {
	require.Equal(t, "alice", fmt.Sprintf("%v", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: "alice"})))
	require.Equal(t, "p1", fmt.Sprintf("%s", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: " "})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.TypographyProfileCompact(nil)))
	require.Equal(t, "alice    |", fmt.Sprintf("%-9v|", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: "alice"})))
	require.Equal(t, "    alice", fmt.Sprintf("%9s", cmdidentity.TypographyProfileCompact(&authn.Profile{Id: "p1", Display: "alice"})))
}

func TestTypographySession(t *testing.T) {
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	profile := &authn.Profile{Id: "p1", AccountId: id, Display: "alice"}

	require.Equal(t, "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b - alice", fmt.Sprintf("%v", cmdidentity.TypographySession(&authn.Current{Profile: profile, Account: &authn.Account{Id: id}})))
	require.Equal(t, "0190a1b2 - acme - alice", fmt.Sprintf("%v", cmdidentity.TypographySession(&authn.Current{Profile: profile, Account: &authn.Account{Id: id, Description: "acme"}})))
	require.Equal(t, "0190a1b2 - acme - alice  |", fmt.Sprintf("%-25v|", cmdidentity.TypographySession(&authn.Current{Profile: profile, Account: &authn.Account{Id: id, Description: "acme"}})))
}

func TestTypographySSHPublicKey(t *testing.T) {
	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded("typography"))
	require.NoError(t, err)

	require.Equal(t, "695e9867-1494-7457-6dc8-0bdb2b848dfc - SHA256:gkCIl+GJjqMhnvT0GcScPW3Rrzst3kywSQ+gTE4vYCM", fmt.Sprintf("%v", cmdidentity.TypographySSHPublicKey(signer.PublicKey())))
	require.Equal(t, "695e9867-1494-7457-6dc8-0bdb2b848dfc - SHA256:gkCIl+", fmt.Sprintf("%.52v", cmdidentity.TypographySSHPublicKey(signer.PublicKey())))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.TypographySSHPublicKey(nil)))
}

func TestTypographySSHPublicKeyB64(t *testing.T) {
	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded("typography"))
	require.NoError(t, err)

	require.Equal(t, "AAAAC3NzaC1lZDI1NTE5AAAAIB-oi5TqWdvwfrnmsBkxpwZtemxQ8WSXvtoLyqf_1Eeo", fmt.Sprintf("%v", cmdidentity.TypographySSHPublicKeyB64(signer.PublicKey())))
	require.Equal(t, "AAAAC3NzaC1l", fmt.Sprintf("%.12v", cmdidentity.TypographySSHPublicKeyB64(signer.PublicKey())))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.TypographySSHPublicKeyB64(nil)))
}
