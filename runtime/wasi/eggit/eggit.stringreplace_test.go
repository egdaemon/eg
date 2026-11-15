package eggit

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/stretchr/testify/require"
)

func TestStringReplace(t *testing.T) {
	ex := testcommit1()
	require.Equal(t, "git hash short: 211a6f2", ex.StringReplace("git hash short: %git.hash.short%"))
	require.Equal(t, "git hash short: 211a6f2", ex.StringReplace("git hash short: {git.hash.short}"))
	require.Equal(t, "git hash: 211a6f2a398230a29830a31a9cbf422be74d407f", ex.StringReplace("git hash: %git.hash%"))
	require.Equal(t, "git commit year: 2006", ex.StringReplace("git commit year: %git.commit.year%"))
	require.Equal(t, "git commit month: 1", ex.StringReplace("git commit month: %git.commit.month%"))
	require.Equal(t, "git commit day: 2", ex.StringReplace("git commit day: %git.commit.day%"))
	require.Equal(t, "git commit seconds: 54245", ex.StringReplace("git commit seconds: {git.commit.seconds}"))
	require.Equal(t, "git commit seconds.short: 54245", ex.StringReplace("git commit seconds.short: {git.commit.seconds.short}"))
	require.Equal(t, "git commit unix: 1136214245", ex.StringReplace("git commit unix: %git.commit.unix%"))
	require.Equal(t, "git commit unix.milli: 1136214245000", ex.StringReplace("git commit unix.milli: %git.commit.unix.milli%"))
	require.Equal(t, "git commit unix.micro: 1136214245000000", ex.StringReplace("git commit unix.micro: %git.commit.unix.micro%"))
}

func TestStringReplaceDateIsUTC(t *testing.T) {
	// 2006-01-02T23:30:00-07:00 is 2006-01-03T06:30:00Z, the date and the time of day must both be the UTC ones.
	ex := testcommit1()
	ex.Committer.When = errorsx.Must(time.Parse(time.RFC3339, "2006-01-02T23:30:00-07:00"))
	require.Equal(t, "2006.1.3.23400", ex.StringReplace("{git.commit.year}.{git.commit.month}.{git.commit.day}.{git.commit.seconds}"))
	require.Equal(t, "1136269800", ex.StringReplace("{git.commit.unix}"))

	// the bounds of the day.
	ex.Committer.When = errorsx.Must(time.Parse(time.RFC3339, "2006-01-02T00:00:00Z"))
	require.Equal(t, "0", ex.StringReplace("{git.commit.seconds}"))
	require.Equal(t, "00000", ex.StringReplace("{git.commit.seconds.short}"))
	ex.Committer.When = errorsx.Must(time.Parse(time.RFC3339, "2006-01-02T23:59:59Z"))
	require.Equal(t, "86399", ex.StringReplace("{git.commit.seconds}"))
	require.Equal(t, "86399", ex.StringReplace("{git.commit.seconds.short}"))
}

func TestStringReplaceSecondsShort(t *testing.T) {
	ex := testcommit1()

	// the fixed width keeps the number following the day sorted by time: 00:08:05 on the 5th is before 22:38:05.
	ex.Committer.When = errorsx.Must(time.Parse(time.RFC3339, "2026-11-05T00:08:05Z"))
	early := ex.StringReplace("{git.commit.day}{git.commit.seconds.short}")
	require.Equal(t, "500485", early)
	ex.Committer.When = errorsx.Must(time.Parse(time.RFC3339, "2026-11-05T22:38:05Z"))
	late := ex.StringReplace("{git.commit.day}{git.commit.seconds.short}")
	require.Equal(t, "581485", late)
	require.Less(t, early, late)

	// the version of the failing extension release: 2026-11-15T22:38:05Z.
	ex.Committer.When = errorsx.Must(time.Parse(time.RFC3339, "2026-11-15T22:38:05Z"))
	require.Equal(t, "2026.11.1581485", ex.StringReplace("{git.commit.year}.{git.commit.month}.{git.commit.day}{git.commit.seconds.short}"))
}
