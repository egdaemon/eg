// Package huhx provides helpers for building huh forms.
package huhx

import (
	"errors"
	"fmt"
	"iter"
	"slices"

	"charm.land/huh/v2"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/termx"
)

// Options builds a huh.Option for each value, labelled by s.
func Options[T comparable](s func(v T) fmt.Formatter, vs ...T) []huh.Option[T] {
	return slices.Collect(OptionsSeq(s, slices.Values(vs)))
}

// OptionsSeq lazily builds a huh.Option for each value yielded by vs, labelled by s.
func OptionsSeq[T comparable](s func(v T) fmt.Formatter, vs iter.Seq[T]) iter.Seq[huh.Option[T]] {
	return func(yield func(huh.Option[T]) bool) {
		for v := range vs {
			if !yield(huh.NewOption(fmt.Sprintf("%v", s(v)), v)) {
				return
			}
		}
	}
}

// Default places def ahead of opts, making it the initially highlighted option.
func Default[T comparable](def huh.Option[T], opts []huh.Option[T]) []huh.Option[T] {
	return slices.Concat([]huh.Option[T]{def}, opts)
}

type prompter struct {
	notty error
}

// Prompter runs interactive prompts, translating the lack of a terminal into a result the caller can act on.
func Prompter() prompter {
	return prompter{notty: termx.ErrNoTTY}
}

// NoTTY sets the error reported, when a prompt is needed but there is no terminal to ask on.
// defaults to termx.ErrNoTTY.
func (t prompter) NoTTY(err error) prompter {
	t.notty = err
	return t
}

// Run runs the prompt, e.g.) a huh field or form.
func (t prompter) Run(r interface{ Run() error }) error {
	err := termx.Run(r)
	if err == nil {
		return nil
	}

	if errors.Is(err, termx.ErrNoTTY) {
		return t.notty
	}

	return errorsx.WithStack(err)
}
