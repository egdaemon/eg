package egnotif

import (
	"bytes"
	"context"
	"errors"
	"text/template"
	"time"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
)

const (
	// DefaultFailure is the markdown text/template used when a run fails and no override is provided.
	DefaultFailure = "**eg workload failed** {{.Workload }} - {{.RunID}}\n\n{{.VCSURI}}@{{.Commit}} after {{.Elapsed}}\n\n{{.Cause}}"
	// DefaultCompleted is the markdown text/template used when a run completes and no override is provided.
	DefaultCompleted = "**eg workload completed** {{.Workload }} - {{.RunID}}\n\n{{.VCSURI}}@{{.Commit}} in {{.Elapsed}}"
)

// Context is the information provided to deliverers about the run.
type Context struct {
	VCSURI    string        // canonical uri of the repository.
	Commit    string        // commit hash being run.
	RunID     string        // id of the run.
	Workload  string        // entry path of the workload, "default" for the default workload.
	AccountID string        // id of the account the run belongs to.
	Initiated time.Time     // when the run started.
	Elapsed   time.Duration // time elapsed since the run started.
	Cause     error         // reason for the failure, nil when the run completed.
	Failure   string        // markdown text/template used when the run failed.
	Completed string        // markdown text/template used when the run completed.
}

// Failed reports whether the run failed.
func (t Context) Failed() bool {
	return t.Cause != nil
}

// Message renders the failure template if the run failed, otherwise the completed template.
func (t Context) Message() (string, error) {
	tmpl := t.Completed
	if t.Failed() {
		tmpl = t.Failure
	}

	return t.Render(tmpl)
}

// Render executes the text/template against the context.
func (t Context) Render(tmpl string) (string, error) {
	var (
		buf bytes.Buffer
	)

	p, err := template.New("egnotif").Parse(tmpl)
	if err != nil {
		return "", err
	}

	if err = p.Execute(&buf, t); err != nil {
		return "", err
	}

	return buf.String(), nil
}

type deliverer interface {
	Send(ctx context.Context, v Context) error
}

// Notifier delivers the outcome of a run.
type Notifier struct {
	d         deliverer
	opts      option
	initiated time.Time
}

// New creates a notifier that sends to d. the run is considered started at the time provided by the runtime,
// see egenv.Initiated.
func New(d deliverer, options ...Option) *Notifier {
	return &Notifier{
		d:         d,
		opts:      langx.Clone(defaults(), options...),
		initiated: egenv.Initiated(),
	}
}

// Fatalln notifies the deliverer of the outcome of the run represented by cause, nil means the run completed.
// cause is returned, joined with any delivery error. it does not exit the process.
func (t *Notifier) Fatalln(ctx context.Context, cause error) error {
	ctx, done := context.WithTimeout(ctx, t.opts.timeout)
	defer done()

	err := t.d.Send(ctx, Context{
		VCSURI:    eggit.EnvCanonicalURI(),
		Commit:    eggit.EnvCommit().Hash.String(),
		RunID:     egenv.RunID(),
		Workload:  stringsx.FirstNonBlank(egenv.Workload(), "default"),
		AccountID: egenv.AccountID(),
		Initiated: t.initiated,
		Elapsed:   time.Since(t.initiated),
		Cause:     cause,
		Failure:   t.opts.failure,
		Completed: t.opts.completed,
	})

	return errors.Join(cause, err)
}
