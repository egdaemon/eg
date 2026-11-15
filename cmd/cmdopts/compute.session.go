package cmdopts

import (
	"github.com/egdaemon/eg/computesess"
	"github.com/egdaemon/eg/internal/uuidx"
)

// ComputeSession is the flag clients use to ask for a run to be registered under an id they can find it by.
// the nil uuid, the default, does not register the run.
type ComputeSession struct {
	Branch  string     `name:"session-branch" help:"branch the run's changes are committed to, saved with the session so resuming it uses the same branch. defaults to the branch the session used before, otherwise a new one is chosen" default:""`
	Session uuidx.UUID `name:"session" help:"uuid to register the run under, tools, i.e. the editor extension, find the run by it. the nil uuid does not register the run" default:"00000000-0000-0000-0000-000000000000"`
}

// AfterApply publishes the flag so the compute commands can take it as a parameter, since the flag is
// declared once on the compute command instead of by each of them.
func (t ComputeSession) AfterApply(dst *ComputeSession) error {
	*dst = t
	return nil
}

// New starts describing the run to register. the builder registers nothing, unless a session was requested
// and the cli's database is available.
func (t ComputeSession) New(db *CLIDB) computesess.Builder {
	if t.Session.IsZero() {
		return computesess.New(nil)
	}

	return computesess.New(db.DB)
}
