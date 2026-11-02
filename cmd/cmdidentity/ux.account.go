package cmdidentity

import (
	"fmt"

	"github.com/egdaemon/eg/authn"
)

type uxAccountFull struct {
	a *authn.Account
}

func (t uxAccountFull) Format(f fmt.State, verb rune) {
	v := dashed(t.a.GetId(), t.a.GetDescription())

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// UXAccountFull renders the account as its id and display name (description), accounts have no email. blank values are omitted.
func UXAccountFull(a *authn.Account) fmt.Formatter {
	return uxAccountFull{a: a}
}
