package gdx

import (
	"time"

	"github.com/egdaemon/gdx/internal/envx"
	"github.com/egdaemon/gdx/internal/langx"
)

type config struct {
	defaultDuration time.Duration
}

type option = func(*config)

// options is a chainable slice of config mutations. each With* method
// appends to the slice and returns the extended slice, so calls compose:
// Options().WithDefaultDuration(x).FromEnv() applies both.
type options []option

// Options returns the zero value options chain.
func Options() options {
	return options(nil)
}

// WithDefaultDuration sets how long a profile/trace capture runs when a
// caller doesn't specify one explicitly (e.g. no ?duration= query parameter).
func (t options) WithDefaultDuration(d time.Duration) options {
	return append(t, func(c *config) { c.defaultDuration = d })
}

// FromEnv reads configuration from the environment:
//   - DIAGX_DEFAULT_DURATION: default profile/trace capture duration, defaults to 10s.
func (t options) FromEnv() options {
	return append(t, func(c *config) {
		c.defaultDuration = envx.Duration(10*time.Second, "DIAGX_DEFAULT_DURATION")
	})
}

func (t options) apply() config {
	return langx.Clone(config{defaultDuration: 10 * time.Second}, t...)
}
