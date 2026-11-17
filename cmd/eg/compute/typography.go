package compute

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
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

// parsets parses an RFC3339 timestamp. blank, unparsable and the infinity sentinels the control plane uses for
// unset timestamps (see timex.RFC3339Inf and timex.RFC3339NegInf) are reported as not set.
func parsets(v string) (time.Time, bool) {
	ts, err := time.Parse(time.RFC3339Nano, v)
	if err != nil || ts.Year() >= timex.RFC3339Inf().Year() || ts.Year() <= timex.RFC3339NegInf().Year() {
		return time.Time{}, false
	}

	return ts, true
}

type workloadCompact struct {
	w *computeapi.Enqueued
}

func (t workloadCompact) Format(f fmt.State, verb rune) {
	v := t.w.GetId()
	if stringsx.Present(t.w.GetDescription()) {
		v = fmt.Sprintf("%s - %s", v, t.w.GetDescription())
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyWorkloadCompact formats the workload as its id when there is no description, otherwise the id
// followed by the description. width, precision and flags are honored.
func TypographyWorkloadCompact(w *computeapi.Enqueued) fmt.Formatter {
	return workloadCompact{w: w}
}

type workloadPlatform struct {
	w *computeapi.Enqueued
}

func (t workloadPlatform) Format(f fmt.State, verb rune) {
	v := fmt.Sprintf("%s/%s", t.w.GetOs(), t.w.GetArch())

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyWorkloadPlatform formats the workload as its operating system followed by its architecture.
func TypographyWorkloadPlatform(w *computeapi.Enqueued) fmt.Formatter {
	return workloadPlatform{w: w}
}

type resultTypography struct {
	r int32
}

func (t resultTypography) Format(f fmt.State, verb rune) {
	v := "?"
	switch t.r {
	case computex.ResultCompleted:
		v = "✔"
	case computex.ResultFailure:
		v = "✖"
	case computex.ResultCancelled:
		v = "⊘"
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyResult formats a computex result as an icon: ✔ completed, ✖ failure, ⊘ cancelled, ? unknown.
func TypographyResult(r int32) fmt.Formatter {
	return resultTypography{r: r}
}

type workloadState struct {
	w *computeapi.Enqueued
}

func (t workloadState) Format(f fmt.State, verb rune) {
	// a completed workload is a result, until then it is either running or queued.
	if _, completed := parsets(t.w.GetCompletedAt()); completed {
		TypographyResult(computex.ResultCompleted).Format(f, verb)
		return
	}

	v := "⏱"
	if _, initiated := parsets(t.w.GetInitiatedAt()); initiated {
		v = "⚙"
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyWorkloadState formats the workload as an icon: ⏱ queued until it is initiated, ⚙ running until it is
// completed, then the completed result, see TypographyResult.
func TypographyWorkloadState(w *computeapi.Enqueued) fmt.Formatter {
	return workloadState{w: w}
}

type historyCompact struct {
	h *compute.History
}

func (t historyCompact) Format(f fmt.State, verb rune) {
	v := t.h.GetId()
	if stringsx.Present(t.h.GetDescription()) {
		v = fmt.Sprintf("%s - %s", v, t.h.GetDescription())
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyHistoryCompact formats the record as its id when there is no description, otherwise the id
// followed by the description. width, precision and flags are honored.
func TypographyHistoryCompact(h *compute.History) fmt.Formatter {
	return historyCompact{h: h}
}

// TypographyHistoryResult formats the outcome of the workload as an icon, see TypographyResult.
func TypographyHistoryResult(h *compute.History) fmt.Formatter {
	return TypographyResult(h.GetResult())
}

type duration struct {
	d time.Duration
}

func (t duration) Format(f fmt.State, verb rune) {
	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), t.d.Round(time.Millisecond).String())
}

// TypographyDuration formats the duration rounded to the millisecond, i.e.) 1m2.345s.
func TypographyDuration(d time.Duration) fmt.Formatter {
	return duration{d: d}
}

// elapsed the time between the start and end timestamps rounded to the second, an end that isn't set is now.
// a start that isn't set is blank.
func elapsed(start, end string) string {
	from, ok := parsets(start)
	if !ok {
		return ""
	}

	to, ok := parsets(end)
	if !ok {
		to = time.Now()
	}

	return to.Sub(from).Round(time.Second).String()
}

type workloadQueued struct {
	w *computeapi.Enqueued
}

func (t workloadQueued) Format(f fmt.State, verb rune) {
	end := t.w.GetInitiatedAt()

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), elapsed(t.w.GetCreatedAt(), end))
}

// TypographyWorkloadQueued formats the time the workload waited in the queue: until it was initiated, or until now
// while it is still waiting.
func TypographyWorkloadQueued(w *computeapi.Enqueued) fmt.Formatter {
	return workloadQueued{w: w}
}

type workloadRunning struct {
	w *computeapi.Enqueued
}

func (t workloadRunning) Format(f fmt.State, verb rune) {
	// blank until the workload has started, elapsed handles an unset start.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), elapsed(t.w.GetInitiatedAt(), t.w.GetCompletedAt()))
}

// TypographyWorkloadRunning formats the time the workload has run: from initiation until completion, or until now
// while it is running. blank when the workload hasn't started.
func TypographyWorkloadRunning(w *computeapi.Enqueued) fmt.Formatter {
	return workloadRunning{w: w}
}

type workloadDeadline struct {
	w *computeapi.Enqueued
}

func (t workloadDeadline) Format(f fmt.State, verb rune) {
	v := ""
	started, ok := parsets(t.w.GetInitiatedAt())
	_, completed := parsets(t.w.GetCompletedAt())
	if ok && !completed {
		remaining := started.Add(time.Duration(t.w.GetTtl())).Sub(time.Now())
		v = max(remaining, 0).Round(time.Second).String()
	}

	// width, precision and flags are applied to the whole value.
	_, _ = fmt.Fprintf(f, fmt.FormatString(f, 's'), v)
}

// TypographyWorkloadDeadline formats the time remaining before the workload exceeds its ttl, counting down to zero from
// initiation. blank when the workload hasn't started or has completed.
func TypographyWorkloadDeadline(w *computeapi.Enqueued) fmt.Formatter {
	return workloadDeadline{w: w}
}
