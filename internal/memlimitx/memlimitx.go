// Package memlimitx provides additional memlimit providers.
package memlimitx

import (
	"errors"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/egdaemon/eg/internal/envx"
)

// FromEnviron is a provider that reads the memory limit from the environment. keys are checked in order
// and the first to parse successfully is used. values may be plain integers or human readable
// sizes such as '6g', '6GB', or '6GiB'. returns memlimit.ErrNoLimit if no key provides a limit.
func FromEnviron(keys ...string) func() (uint64, error) {
	return func() (uint64, error) {
		if limit := envx.Bytes(0, keys...); limit > 0 {
			return limit, nil
		}

		return 0, memlimit.ErrNoLimit
	}
}

// Chain is an option that configures memlimit.Set with the first provider, in the order given, that reports
// a limit. providers returning memlimit.ErrNoLimit are skipped, any other error is returned immediately.
// if no provider reports a limit the result is memlimit.ErrNoLimit.
//
// like memlimit.WithProvider it replaces any previously configured provider.
func Chain(providers ...memlimit.Provider) memlimit.Option {
	return memlimit.WithProvider(func() (uint64, error) {
		for _, p := range providers {
			limit, err := p()
			if errors.Is(err, memlimit.ErrNoLimit) {
				continue
			}

			if err != nil {
				return 0, err
			}

			return limit, nil
		}

		return 0, memlimit.ErrNoLimit
	})
}
