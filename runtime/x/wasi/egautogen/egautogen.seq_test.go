package egautogen

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSeqLimit(t *testing.T) {
	fns := []Fn{{Name: "One"}, {Name: "Two"}, {Name: "Three"}}

	t.Run("yields at most n functions", func(t *testing.T) {
		s := FromSpecified(fns...).Limit(2)

		require.Equal(t, fns[:2], slices.Collect(s.Each(t.Context())))
		require.NoError(t, s.Err())
	})

	t.Run("zero yields every function", func(t *testing.T) {
		s := FromSpecified(fns...).Limit(0)

		require.Equal(t, fns, slices.Collect(s.Each(t.Context())))
	})
}

func TestSeqUntil(t *testing.T) {
	fns := []Fn{{Name: "One"}, {Name: "Two"}, {Name: "Three"}}

	t.Run("yields every function without a deadline", func(t *testing.T) {
		s := FromSpecified(fns...).Until(time.Hour)

		require.Equal(t, fns, slices.Collect(s.Each(t.Context())))
		require.NoError(t, s.Err())
	})

	t.Run("yields nothing within the grace of the deadline", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), time.Minute)
		defer done()

		s := FromSpecified(fns...).Until(time.Hour)

		require.Empty(t, slices.Collect(s.Each(ctx)))
	})

	t.Run("yields every function before the grace of the deadline", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), time.Hour)
		defer done()

		s := FromSpecified(fns...).Until(time.Minute)

		require.Equal(t, fns, slices.Collect(s.Each(ctx)))
	})
}
