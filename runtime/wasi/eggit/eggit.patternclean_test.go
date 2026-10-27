package eggit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatternClean(t *testing.T) {
	require.Equal(t, "", PatternClean("%git.hash%"))
	require.Equal(t, "", PatternClean("%git.hash.short%"))
	require.Equal(t, "", PatternClean("%git.commit.year%"))
	require.Equal(t, "", PatternClean("%git.commit.month%"))
	require.Equal(t, "", PatternClean("%git.commit.day%"))
	require.Equal(t, "", PatternClean("%git.commit.unix%"))
	require.Equal(t, "", PatternClean("%git.commit.unix.milli%"))
	require.Equal(t, "", PatternClean("%git.commit.unix.micro%"))
	require.Equal(t, "", PatternClean("%eg.git.canonical.uri%"))
	// the .% prefix is stripped to avoid leaving a dangling separator
	require.Equal(t, "v1", PatternClean("v1.%git.commit.unix%"))
	// the %. suffix is stripped to avoid leaving a dangling separator
	require.Equal(t, "v1", PatternClean("%git.commit.unix%.v1"))
}
