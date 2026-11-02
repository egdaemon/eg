package cmdidentity

import (
	"fmt"

	"github.com/egdaemon/eg/authn"
)

type uxIdentityFull struct {
	i *authn.Identity
}

func (t uxIdentityFull) Format(f fmt.State, verb rune) {
	v := dashed(t.i.GetUid(), t.i.GetDisplay(), t.i.GetEmail())

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// UXIdentityFull renders the identity as its uid, display name and email. blank values are omitted.
func UXIdentityFull(i *authn.Identity) fmt.Formatter {
	return uxIdentityFull{i: i}
}
