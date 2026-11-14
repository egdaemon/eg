package cmdtypography_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/cmdtypography"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/stretchr/testify/require"
)

func TestTimestamp(t *testing.T) {
	ts := time.Date(2026, time.November, 14, 6, 30, 0, 0, time.UTC)

	t.Run("formats the time in utc as RFC3339Nano", func(t *testing.T) {
		local := time.Date(2026, time.November, 14, 6, 30, 0, 123, time.FixedZone("est", -5*60*60))
		require.Equal(t, "2026-11-14T11:30:00.000000123Z", fmt.Sprintf("%v", cmdtypography.Timestamp(local).Location(time.UTC)))
	})

	t.Run("layout", func(t *testing.T) {
		require.Equal(t, "2026-11-14", fmt.Sprintf("%v", cmdtypography.Timestamp(ts).Location(time.UTC).Layout(time.DateOnly)))
	})

	t.Run("zero is the text asked for", func(t *testing.T) {
		require.Equal(t, "", fmt.Sprintf("%v", cmdtypography.Timestamp(time.Time{}).Location(time.UTC).Zero("")))
		require.Equal(t, "n/a", fmt.Sprintf("%v", cmdtypography.Timestamp(time.Time{}).Location(time.UTC).Zero("n/a")))
		require.Equal(t, "0001-01-01T00:00:00Z", fmt.Sprintf("%v", cmdtypography.Timestamp(time.Time{}).Location(time.UTC)), "the time itself when no text is asked for")
	})

	t.Run("infinity is the text asked for", func(t *testing.T) {
		f := func(ts time.Time) string {
			return fmt.Sprintf("%v", cmdtypography.Timestamp(ts).Location(time.UTC).Zero("").Infinity("inf", "-inf"))
		}

		require.Equal(t, "inf", f(timex.Inf()))
		require.Equal(t, "inf", f(timex.RFC3339Inf()))
		require.Equal(t, "-inf", f(timex.NegInf()))
		require.Equal(t, "-inf", f(timex.RFC3339NegInf()))
		require.Equal(t, "", f(time.Time{}), "zero is not an infinity")
		require.Equal(t, "2026-11-14T06:30:00Z", f(ts))
	})

	t.Run("infinity is the clamped time when no text is asked for", func(t *testing.T) {
		require.Equal(t, "9999-12-31T23:59:59.999Z", fmt.Sprintf("%v", cmdtypography.Timestamp(timex.Inf()).Location(time.UTC)))
	})

	t.Run("location", func(t *testing.T) {
		est := time.FixedZone("est", -5*60*60)
		require.Equal(t, "2026-11-14 01:30:00", fmt.Sprintf("%v", cmdtypography.Timestamp(ts).Layout(time.DateTime).Location(est)))
	})

	t.Run("reads rfc3339 text", func(t *testing.T) {
		f := func(v string) string {
			return fmt.Sprintf("%v", cmdtypography.TimestampRFC3339(v).Location(time.UTC).Layout(time.DateTime).Zero("").Infinity("", ""))
		}

		require.Equal(t, "2026-11-14 06:30:00", f("2026-11-14T06:30:00Z"))
		require.Equal(t, "", f("9999-12-31T23:59:59.999Z"), "infinity is not set")
		require.Equal(t, "", f("0000-01-01T01:01:01Z"), "negative infinity is not set")
		require.Equal(t, "", f(""), "blank is printed as is")
		require.Equal(t, "garbage", f("garbage"), "unparsable text is printed as is")
	})

	t.Run("honors width and precision", func(t *testing.T) {
		require.Equal(t, "2026-11-14T06:30:00Z    |", fmt.Sprintf("%-24v|", cmdtypography.Timestamp(ts).Location(time.UTC)))
		require.Equal(t, "2026-11-14", fmt.Sprintf("%.10v", cmdtypography.Timestamp(ts).Location(time.UTC)))
		require.Equal(t, "inf   |", fmt.Sprintf("%-6v|", cmdtypography.Timestamp(timex.Inf()).Location(time.UTC).Infinity("inf", "-inf")))
	})
}
