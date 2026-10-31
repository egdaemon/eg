package actl

import (
	"fmt"

	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners/registration"
)

type registrationCompact struct {
	r *registration.Registration
}

func (t registrationCompact) Format(f fmt.State, verb rune) {
	v := fmt.Sprintf("%.8s - %s", t.r.GetId(), stringsx.DefaultIfBlank(t.r.GetDescription(), "no description"))

	if stringsx.Present(t.r.GetAuthzedAt()) {
		v = fmt.Sprintf("%s - authorized %s", v, t.r.GetAuthzedAt())
	}

	if stringsx.Present(t.r.GetExpiresAt()) {
		v = fmt.Sprintf("%s - expires %s", v, t.r.GetExpiresAt())
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyRegistrationCompact formats the registration as its abbreviated id and description, followed by when it was
// authorized and when it expires if they are known. width, precision and flags are honored.
func TypographyRegistrationCompact(r *registration.Registration) fmt.Formatter {
	return registrationCompact{r: r}
}
