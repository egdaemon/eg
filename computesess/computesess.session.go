package computesess

import "github.com/egdaemon/eg/internal/timex"

// Running reports whether the session has not been stopped: it stops at infinity.
func (t Session) Running() bool {
	return !t.StoppedAt.Before(timex.RFC3339Inf())
}
