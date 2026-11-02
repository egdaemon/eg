package cmdidentity

import (
	"fmt"

	"github.com/egdaemon/eg/authn"
)

type uxProfileFull struct {
	p *authn.Profile
}

func (t uxProfileFull) Format(f fmt.State, verb rune) {
	v := dashed(t.p.GetId(), t.p.GetDisplay(), t.p.GetEmail())

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// UXProfileFull renders the profile as its id, display name and email. blank values are omitted.
func UXProfileFull(p *authn.Profile) fmt.Formatter {
	return uxProfileFull{p: p}
}
