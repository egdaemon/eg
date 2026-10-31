package cmdidentity

import (
	"encoding/base64"
	"fmt"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
)

type accountCompact struct {
	a *authn.Account
}

func (t accountCompact) Format(f fmt.State, verb rune) {
	v := t.a.GetId()
	if stringsx.Present(t.a.GetDescription()) {
		v = fmt.Sprintf("%.8s - %s", v, t.a.GetDescription())
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyAccountCompact formats the account as its id when there is no display name, otherwise the abbreviated id
// followed by the display name. width, precision and flags are honored.
func TypographyAccountCompact(a *authn.Account) fmt.Formatter {
	return accountCompact{a: a}
}

type profileCompact struct {
	p *authn.Profile
}

func (t profileCompact) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), stringsx.First(t.p.GetDisplay(), t.p.GetId()))
}

// TypographyProfileCompact formats the profile as its shortest human friendly name, the display name falling back to the id.
func TypographyProfileCompact(p *authn.Profile) fmt.Formatter {
	return profileCompact{p: p}
}

type authnTypography struct {
	a *authn.Authn
}

func (t authnTypography) Format(f fmt.State, verb rune) {
	v := fmt.Sprintf("%v - %v", TypographyAccountCompact(t.a.Account), TypographyProfileCompact(t.a.Profile))

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyAuthn formats the identity as its account followed by its profile. both are required.
func TypographyAuthn(a *authn.Authn) fmt.Formatter {
	return authnTypography{a: a}
}

type sessionTypography struct {
	c *authn.Current
}

func (t sessionTypography) Format(f fmt.State, verb rune) {
	v := fmt.Sprintf("%v - %v", TypographyAccountCompact(t.c.Account), TypographyProfileCompact(t.c.Profile))

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographySession formats the session as its account followed by its profile. both are required.
func TypographySession(c *authn.Current) fmt.Formatter {
	return sessionTypography{c: c}
}

type sshPublicKey struct {
	k ssh.PublicKey
}

func (t sshPublicKey) Format(f fmt.State, verb rune) {
	var v string
	if t.k != nil {
		v = fmt.Sprintf("%s - %s", identity.ID(t.k), ssh.FingerprintSHA256(t.k))
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographySSHPublicKey formats the ssh public key as its identity followed by its fingerprint.
func TypographySSHPublicKey(k ssh.PublicKey) fmt.Formatter {
	return sshPublicKey{k: k}
}

type sshPublicKeyB64 struct {
	k ssh.PublicKey
}

func (t sshPublicKeyB64) Format(f fmt.State, verb rune) {
	var v string
	if t.k != nil {
		v = base64.URLEncoding.EncodeToString(t.k.Marshal())
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographySSHPublicKeyB64 formats the ssh public key as its url safe base64 encoded wire format.
func TypographySSHPublicKeyB64(k ssh.PublicKey) fmt.Formatter {
	return sshPublicKeyB64{k: k}
}
