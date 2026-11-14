// Package cmdtypography formats values for the terminal and for tools reading eg's output.
package cmdtypography

import (
	"fmt"
	"time"

	"github.com/egdaemon/eg/internal/timex"
)

// timestamp formats a time. a time that is not set and the infinities, which the console's time components show as
// never and always, are written as their own text when asked, otherwise as the layout of the time they represent.
type timestamp struct {
	t      time.Time
	layout string
	loc    *time.Location
	zero   *string
	inf    *string
	neginf *string
	// raw is printed as is when the text the time was parsed from isn't a time.
	raw      string
	unparsed bool
}

// Timestamp formats the time in local time as RFC3339Nano. use Layout, Location, Zero and Infinity to control the output.
//
//	fmt.Sprint(cmdtypography.Timestamp(t).Zero("").Infinity("never", "always"))
//
// width, precision and flags are honored.
func Timestamp(t time.Time) timestamp {
	return timestamp{t: t, layout: time.RFC3339Nano, loc: time.Local}
}

// TimestampRFC3339 formats a time read from its RFC3339 text, i.e. the timestamps of the control plane. text that is not a
// time is printed as is. see Timestamp.
func TimestampRFC3339(v string) timestamp {
	t := Timestamp(time.Time{})

	parsed, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		t.raw, t.unparsed = v, true
		return t
	}

	t.t = parsed
	return t
}

// Layout sets the layout of the time, see time.Time.Format.
func (t timestamp) Layout(layout string) timestamp {
	t.layout = layout
	return t
}

// Location sets the location the time is shown in.
func (t timestamp) Location(loc *time.Location) timestamp {
	t.loc = loc
	return t
}

// Zero sets the text for a time that is not set.
func (t timestamp) Zero(text string) timestamp {
	t.zero = &text
	return t
}

// Infinity sets the text for positive and negative infinity.
func (t timestamp) Infinity(positive string, negative string) timestamp {
	t.inf = &positive
	t.neginf = &negative
	return t
}

func (t timestamp) Format(f fmt.State, verb rune) {
	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), t.text())
}

func (t timestamp) text() string {
	switch {
	case t.unparsed:
		return t.raw
	case t.t.IsZero() && t.zero != nil:
		return *t.zero
	case !t.t.Before(timex.RFC3339Inf()) && t.inf != nil:
		return *t.inf
	case !t.t.After(timex.RFC3339NegInf()) && !t.t.IsZero() && t.neginf != nil:
		return *t.neginf
	default:
		return timex.RFC3339NanoEncode(t.t).In(t.loc).Format(t.layout)
	}
}
