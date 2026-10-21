package egnotif

import "time"

const defaultTimeout = 30 * time.Second

// Option configures the notifier.
type Option func(*option)
type options []Option

type option struct {
	failure   string
	completed string
	timeout   time.Duration
}

func defaults() option {
	return option{
		failure:   DefaultFailure,
		completed: DefaultCompleted,
		timeout:   defaultTimeout,
	}
}

// Options generates the default options. all options are optional.
func Options() options {
	return options(nil)
}

// Failure overrides the markdown text/template used when the run fails.
func (t options) Failure(tmpl string) options {
	return append(t, func(o *option) {
		o.failure = tmpl
	})
}

// Completed overrides the markdown text/template used when the run completes.
func (t options) Completed(tmpl string) options {
	return append(t, func(o *option) {
		o.completed = tmpl
	})
}

// Timeout bounds how long delivery may take.
func (t options) Timeout(d time.Duration) options {
	return append(t, func(o *option) {
		o.timeout = d
	})
}
