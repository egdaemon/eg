package cmdidentity_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/stretchr/testify/require"
)

func TestUXProfileFull(t *testing.T) {
	require.Equal(t, "p1 - alice - alice@example.com", fmt.Sprintf("%v", cmdidentity.UXProfileFull(&authn.Profile{Id: "p1", Display: "alice", Email: "alice@example.com"})))
	require.Equal(t, "p1", fmt.Sprintf("%v", cmdidentity.UXProfileFull(&authn.Profile{Id: "p1"})))
	require.Equal(t, "p1 - alice  |", fmt.Sprintf("%-12v|", cmdidentity.UXProfileFull(&authn.Profile{Id: "p1", Display: "alice"})))
	require.Equal(t, "", fmt.Sprintf("%v", cmdidentity.UXProfileFull(nil)))
}
