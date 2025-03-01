package eggit

import (
	"fmt"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/stretchr/testify/require"
)

func testcommit1() commit {
	return commit{
		Hash: nhash("211a6f2a398230a29830a31a9cbf422be74d407f"),
		Committer: signature{
			Name:  "EGd engineering",
			Email: "engineering@egdaemon.com",
			When:  errorsx.Must(time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")),
		},
	}
}

func TestHashFormat(t *testing.T) {
	const (
		fulllength     = "211a6f2a398230a29830a31a9cbf422be74d407f"
		customlength1  = "2"
		customlength2  = "21"
		customlength6  = "211a6f"
		shortlength    = "211a6f2"
		customlength14 = "211a6f2a398230"
		customlength15 = "211a6f2a398230a"
		customlength19 = "211a6f2a398230a2983"
		customlength20 = "211a6f2a398230a29830"
	)

	ex := nhash(fulllength)
	require.Equal(t, customlength1, fmt.Sprintf("%1s", ex))
	require.Equal(t, customlength2, fmt.Sprintf("%2s", ex))

	require.Equal(t, customlength6, fmt.Sprintf("%6s", ex))
	require.Equal(t, shortlength, fmt.Sprintf("%7s", ex))

	require.Equal(t, customlength14, fmt.Sprintf("%14s", ex))
	require.Equal(t, customlength15, fmt.Sprintf("%15s", ex))

	require.Equal(t, customlength19, fmt.Sprintf("%19s", ex))
	require.Equal(t, customlength20, fmt.Sprintf("%20s", ex))
	require.Equal(t, fulllength, fmt.Sprintf("%s", ex))
}

func TestStringReplace(t *testing.T) {
	ex := testcommit1()
	require.Equal(t, "git hash short: 211a6f2", ex.StringReplace("git hash short: %git.hash.short%"))
	require.Equal(t, "git hash: 211a6f2a398230a29830a31a9cbf422be74d407f", ex.StringReplace("git hash: %git.hash%"))
	require.Equal(t, "git commit year: 2006", ex.StringReplace("git commit year: %git.commit.year%"))
	require.Equal(t, "git commit month: 1", ex.StringReplace("git commit month: %git.commit.month%"))
	require.Equal(t, "git commit day: 2", ex.StringReplace("git commit day: %git.commit.day%"))
	require.Equal(t, "git commit unix.milli: 1136214245000", ex.StringReplace("git commit unix.milli: %git.commit.unix.milli%"))
}
