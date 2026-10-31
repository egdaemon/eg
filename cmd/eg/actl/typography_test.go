package actl_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/stretchr/testify/require"
)

func TestTypographyRegistrationCompact(t *testing.T) {
	const id = "0190a1b2c3d47e5f8a9b0c1d2e3f4a5b"

	require.Equal(t, "0190a1b2 - no description", fmt.Sprintf("%v", actl.TypographyRegistrationCompact(&registration.Registration{Id: id})))
	require.Equal(t, "0190a1b2 - builder", fmt.Sprintf("%v", actl.TypographyRegistrationCompact(&registration.Registration{Id: id, Description: "builder"})))
	require.Equal(t, "0190a1b2 - builder - authorized 2026-10-31T17:00:00Z", fmt.Sprintf("%v", actl.TypographyRegistrationCompact(&registration.Registration{Id: id, Description: "builder", AuthzedAt: "2026-10-31T17:00:00Z"})))
	require.Equal(t, "0190a1b2 - builder - authorized 2026-10-31T17:00:00Z - expires 2026-11-30T17:00:00Z", fmt.Sprintf("%s", actl.TypographyRegistrationCompact(&registration.Registration{Id: id, Description: "builder", AuthzedAt: "2026-10-31T17:00:00Z", ExpiresAt: "2026-11-30T17:00:00Z"})))
	require.Equal(t, "0190a1b2 - build", fmt.Sprintf("%.16v", actl.TypographyRegistrationCompact(&registration.Registration{Id: id, Description: "builder"})))
	require.Equal(t, " - no description", fmt.Sprintf("%v", actl.TypographyRegistrationCompact(nil)))
	require.Equal(t, "0190a1b2 - builder  |", fmt.Sprintf("%-20v|", actl.TypographyRegistrationCompact(&registration.Registration{Id: id, Description: "builder"})))
	require.Equal(t, "  0190a1b2 - builder", fmt.Sprintf("%20s", actl.TypographyRegistrationCompact(&registration.Registration{Id: id, Description: "builder"})))
}
