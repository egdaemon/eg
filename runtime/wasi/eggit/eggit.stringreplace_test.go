package eggit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringReplace(t *testing.T) {
	ex := testcommit1()
	require.Equal(t, "git hash short: 211a6f2", ex.StringReplace("git hash short: %git.hash.short%"))
	require.Equal(t, "git hash: 211a6f2a398230a29830a31a9cbf422be74d407f", ex.StringReplace("git hash: %git.hash%"))
	require.Equal(t, "git commit year: 2006", ex.StringReplace("git commit year: %git.commit.year%"))
	require.Equal(t, "git commit month: 1", ex.StringReplace("git commit month: %git.commit.month%"))
	require.Equal(t, "git commit day: 2", ex.StringReplace("git commit day: %git.commit.day%"))
	require.Equal(t, "git commit unix: 1136214245", ex.StringReplace("git commit unix: %git.commit.unix%"))
	require.Equal(t, "git commit unix.milli: 1136214245000", ex.StringReplace("git commit unix.milli: %git.commit.unix.milli%"))
	require.Equal(t, "git commit unix.micro: 1136214245000000", ex.StringReplace("git commit unix.micro: %git.commit.unix.micro%"))
}
