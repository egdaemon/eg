package egautogen

import (
	"time"

	"github.com/egdaemon/eg/internal/iterx"
)

// Seq is the functions to generate tests for.
type Seq struct {
	iterx.Seq[Fn]
}

// Limit bounds the functions to at most n of them, e.g.) the 5 least covered of Worst. filters apply before
// the limit, so the limit is of the functions that match. 0 is all of them.
func (t Seq) Limit(n uint) Seq {
	return Seq{iterx.N(n, t.Seq)}
}

// Until stops the functions once less than grace remains before the deadline of the context they are consumed with,
// leaving the function in flight time to finish. a context without a deadline yields every function.
func (t Seq) Until(grace time.Duration) Seq {
	return Seq{iterx.Until(grace, t.Seq)}
}
