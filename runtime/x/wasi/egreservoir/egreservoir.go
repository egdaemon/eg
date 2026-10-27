// Package egreservoir provides reservoir sampling over sequences.
package egreservoir

import (
	"context"
	"math/rand/v2"

	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/iterx"
)

// Sampler samples sequences of T using randomness derived from a seed.
type Sampler[T any] struct {
	seed string
}

// Seeded returns a Sampler whose randomness is derived from seed,
// the same seed over the same sequence yields the same sample.
func Seeded[T any](seed string) Sampler[T] {
	return Sampler[T]{seed: seed}
}

// Sample returns a Seq yielding a uniform random sample of up to n items from seq
// (reservoir sampling, Algorithm R). seq is consumed fully, memory is O(n), and the
// order of the yielded items is unspecified.
func (t Sampler[T]) Sample(n int, seq iterx.Seq[T]) iterx.Seq[T] {
	return iterx.New(func(ctx context.Context, yield func(T) bool) error {
		if n <= 0 {
			return nil
		}

		rng := rand.New(cryptox.NewChaCha8(t.seed))
		reservoir := make([]T, 0, n)
		seen := 0
		for v := range seq.Each(ctx) {
			if len(reservoir) < n {
				reservoir = append(reservoir, v)
			} else if j := rng.IntN(seen + 1); j < n {
				reservoir[j] = v
			}
			seen++
		}

		if err := seq.Err(); err != nil {
			return err
		}

		for _, v := range reservoir {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if !yield(v) {
				return nil
			}
		}

		return nil
	})
}
