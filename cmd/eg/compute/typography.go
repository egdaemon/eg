package compute

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/stringsx"
)

type runnerCompact struct {
	r *compute.Compute
}

func (t runnerCompact) Format(f fmt.State, verb rune) {
	v := t.r.GetId()
	if stringsx.Present(t.r.GetDescription()) {
		v = fmt.Sprintf("%s - %s", v, t.r.GetDescription())
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyRunnerCompact formats the runner as its id when there is no description, otherwise the id
// followed by the description. width, precision and flags are honored.
func TypographyRunnerCompact(r *compute.Compute) fmt.Formatter {
	return runnerCompact{r: r}
}

type runnerPlatform struct {
	r *compute.Compute
}

func (t runnerPlatform) Format(f fmt.State, verb rune) {
	v := fmt.Sprintf("%s/%s", t.r.GetOs(), t.r.GetArch())

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyRunnerPlatform formats the runner as its operating system followed by its architecture.
func TypographyRunnerPlatform(r *compute.Compute) fmt.Formatter {
	return runnerPlatform{r: r}
}

type runnerLabels struct {
	r *compute.Compute
}

func (t runnerLabels) Format(f fmt.State, verb rune) {
	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), strings.Join(t.r.GetLabels(), ","))
}

// TypographyRunnerLabels formats the runner's labels as a comma separated list.
func TypographyRunnerLabels(r *compute.Compute) fmt.Formatter {
	return runnerLabels{r: r}
}

type runnerLastSeen struct {
	r *compute.Compute
}

func (t runnerLastSeen) Format(f fmt.State, verb rune) {
	v := t.r.GetUpdatedAt()
	if ts, err := time.Parse(time.RFC3339Nano, v); err == nil {
		v = ts.Local().Format(time.DateTime)
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyRunnerLastSeen formats when the runner was last seen in local time, an unparsable value is printed as is.
func TypographyRunnerLastSeen(r *compute.Compute) fmt.Formatter {
	return runnerLastSeen{r: r}
}

type bytesTypography struct {
	n uint64
}

func (t bytesTypography) Format(f fmt.State, verb rune) {
	var (
		div    = float64(1)
		suffix = "B"
		prec   = 2
	)

	switch {
	case t.n >= bytesx.EiB:
		div, suffix = bytesx.EiB, "EiB"
	case t.n >= bytesx.PiB:
		div, suffix = bytesx.PiB, "PiB"
	case t.n >= bytesx.TiB:
		div, suffix = bytesx.TiB, "TiB"
	case t.n >= bytesx.GiB:
		div, suffix = bytesx.GiB, "GiB"
	case t.n >= bytesx.MiB:
		div, suffix = bytesx.MiB, "MiB"
	case t.n >= bytesx.KiB:
		div, suffix = bytesx.KiB, "KiB"
	}

	if p, ok := f.Precision(); ok {
		prec = p
	}

	// bytes are whole numbers.
	if suffix == "B" {
		prec = 0
	}

	v := strconv.FormatFloat(float64(t.n)/div, 'f', prec, 64) + suffix

	// the precision is consumed by the number, only the width and alignment are applied to the whole value.
	width, _ := f.Width()
	if f.Flag('-') {
		width = -width
	}
	_, _ = fmt.Fprintf(f, "%*s", width, v)
}

// TypographyBytes formats the size using the largest binary unit it reaches, i.e.) 4.32GiB. precision is the number of
// decimals (default 2), sizes below 1KiB are whole bytes. width and the left align flag are honored.
func TypographyBytes(n uint64) fmt.Formatter {
	return bytesTypography{n: n}
}
