package cmdidentity

import (
	"fmt"

	"github.com/egdaemon/eg/authn"
)

type uxSessionFull struct {
	i *authn.Identity
	a *authn.Authn
}

func (t uxSessionFull) Format(f fmt.State, verb rune) {
	v := fmt.Sprintf("identity: %v\naccount: %v\nprofile: %v", UXIdentityFull(t.i), UXAccountFull(t.a.GetAccount()), UXProfileFull(t.a.GetProfile()))

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// UXSessionFull renders the session as three lines, the identity, the profile, and the account, each in full.
func UXSessionFull(i *authn.Identity, a *authn.Authn) fmt.Formatter {
	return uxSessionFull{i: i, a: a}
}
