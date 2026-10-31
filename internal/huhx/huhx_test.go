package huhx_test

import (
	"errors"
	"os/exec"
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/termx"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/internal/huhx"
	"github.com/stretchr/testify/require"
)

func TestOptions(t *testing.T) {
	alice := &authn.Profile{Id: "p1", Display: "alice"}
	bob := &authn.Profile{Id: "p2"}

	opts := huhx.Options(cmdidentity.TypographyProfileCompact, alice, bob)
	require.Len(t, opts, 2)
	require.Equal(t, "alice", opts[0].Key)
	require.Equal(t, alice, opts[0].Value)
	require.Equal(t, "p2", opts[1].Key)
	require.Equal(t, bob, opts[1].Value)
	require.Empty(t, huhx.Options(cmdidentity.TypographyProfileCompact))
}

func TestOptionsSeqStopsEarly(t *testing.T) {
	var n int
	for o := range huhx.OptionsSeq(cmdidentity.TypographyProfileCompact, slices.Values([]*authn.Profile{{Id: "p1"}, {Id: "p2"}})) {
		require.Equal(t, "p1", o.Key)
		n++
		break
	}
	require.Equal(t, 1, n)
}

func TestPrompter(t *testing.T) {
	const ambiguous = errorsx.String("ambiguous")

	// exec.Cmd stands in for a prompt, it has the same Run() error shape.
	err := huhx.Prompter().NoTTY(ambiguous).Run(exec.Command("/nonexistent/prompt"))
	require.ErrorIs(t, err, ambiguous)

	err = huhx.Prompter().Run(exec.Command("/nonexistent/prompt"))
	require.ErrorIs(t, err, termx.ErrNoTTY)

	err = huhx.Prompter().NoTTY(ambiguous).Run(exec.Command("false"))
	require.Error(t, err)
	require.False(t, errors.Is(err, ambiguous))

	require.NoError(t, huhx.Prompter().Run(exec.Command("true")))
}
