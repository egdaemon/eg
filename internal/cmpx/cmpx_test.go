package cmpx_test

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/cmpx"
	"github.com/stretchr/testify/require"
)

// TestCmpOrdered covers every Ordered kind, for the builtin type and for a named type with that underlying type.
func TestCmpOrdered(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[int]().Cmp(func(v int) int { return v })
		rev := cmpx.New[int]().Rev(func(v int) int { return v })
		vals := []int{math.MinInt64, math.MinInt64 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt64 - 1, math.MaxInt64}
		ascending := make([]int, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := int(rng.Uint64()), int(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int named", func(t *testing.T) {
		type named int
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []int{math.MinInt64, math.MinInt64 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt64 - 1, math.MaxInt64}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(int(rng.Uint64())), named(int(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int8", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[int8]().Cmp(func(v int8) int8 { return v })
		rev := cmpx.New[int8]().Rev(func(v int8) int8 { return v })
		vals := []int8{math.MinInt8, math.MinInt8 + 1, -1, 0, 1, 127 - 1, math.MaxInt8}
		ascending := make([]int8, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := int8(rng.Uint64()), int8(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int8 named", func(t *testing.T) {
		type named int8
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []int8{math.MinInt8, math.MinInt8 + 1, -1, 0, 1, 127 - 1, math.MaxInt8}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(int8(rng.Uint64())), named(int8(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int16", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[int16]().Cmp(func(v int16) int16 { return v })
		rev := cmpx.New[int16]().Rev(func(v int16) int16 { return v })
		vals := []int16{math.MinInt16, math.MinInt16 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt16 - 1, math.MaxInt16}
		ascending := make([]int16, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := int16(rng.Uint64()), int16(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int16 named", func(t *testing.T) {
		type named int16
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []int16{math.MinInt16, math.MinInt16 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt16 - 1, math.MaxInt16}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(int16(rng.Uint64())), named(int16(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int32", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[int32]().Cmp(func(v int32) int32 { return v })
		rev := cmpx.New[int32]().Rev(func(v int32) int32 { return v })
		vals := []int32{math.MinInt32, math.MinInt32 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt32 - 1, math.MaxInt32}
		ascending := make([]int32, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := int32(rng.Uint64()), int32(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int32 named", func(t *testing.T) {
		type named int32
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []int32{math.MinInt32, math.MinInt32 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt32 - 1, math.MaxInt32}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(int32(rng.Uint64())), named(int32(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int64", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[int64]().Cmp(func(v int64) int64 { return v })
		rev := cmpx.New[int64]().Rev(func(v int64) int64 { return v })
		vals := []int64{math.MinInt64, math.MinInt64 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt64 - 1, math.MaxInt64}
		ascending := make([]int64, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := int64(rng.Uint64()), int64(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("int64 named", func(t *testing.T) {
		type named int64
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []int64{math.MinInt64, math.MinInt64 + 1, -256, -1, 0, 1, 255, 256, math.MaxInt64 - 1, math.MaxInt64}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(int64(rng.Uint64())), named(int64(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[uint]().Cmp(func(v uint) uint { return v })
		rev := cmpx.New[uint]().Rev(func(v uint) uint { return v })
		vals := []uint{0, 1, 255, 256, math.MaxUint64 - 1, math.MaxUint64}
		ascending := make([]uint, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := uint(rng.Uint64()), uint(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint named", func(t *testing.T) {
		type named uint
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []uint{0, 1, 255, 256, math.MaxUint64 - 1, math.MaxUint64}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(uint(rng.Uint64())), named(uint(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint8", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[uint8]().Cmp(func(v uint8) uint8 { return v })
		rev := cmpx.New[uint8]().Rev(func(v uint8) uint8 { return v })
		vals := []uint8{0, 1, 127, 128, 254, math.MaxUint8}
		ascending := make([]uint8, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := uint8(rng.Uint64()), uint8(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint8 named", func(t *testing.T) {
		type named uint8
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []uint8{0, 1, 127, 128, 254, math.MaxUint8}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(uint8(rng.Uint64())), named(uint8(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint16", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[uint16]().Cmp(func(v uint16) uint16 { return v })
		rev := cmpx.New[uint16]().Rev(func(v uint16) uint16 { return v })
		vals := []uint16{0, 1, 255, 256, math.MaxUint16 - 1, math.MaxUint16}
		ascending := make([]uint16, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := uint16(rng.Uint64()), uint16(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint16 named", func(t *testing.T) {
		type named uint16
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []uint16{0, 1, 255, 256, math.MaxUint16 - 1, math.MaxUint16}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(uint16(rng.Uint64())), named(uint16(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint32", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[uint32]().Cmp(func(v uint32) uint32 { return v })
		rev := cmpx.New[uint32]().Rev(func(v uint32) uint32 { return v })
		vals := []uint32{0, 1, 255, 256, math.MaxUint32 - 1, math.MaxUint32}
		ascending := make([]uint32, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := uint32(rng.Uint64()), uint32(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint32 named", func(t *testing.T) {
		type named uint32
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []uint32{0, 1, 255, 256, math.MaxUint32 - 1, math.MaxUint32}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(uint32(rng.Uint64())), named(uint32(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint64", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[uint64]().Cmp(func(v uint64) uint64 { return v })
		rev := cmpx.New[uint64]().Rev(func(v uint64) uint64 { return v })
		vals := []uint64{0, 1, 255, 256, math.MaxUint64 - 1, math.MaxUint64}
		ascending := make([]uint64, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := rng.Uint64(), rng.Uint64()
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uint64 named", func(t *testing.T) {
		type named uint64
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []uint64{0, 1, 255, 256, math.MaxUint64 - 1, math.MaxUint64}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(rng.Uint64()), named(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uintptr", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[uintptr]().Cmp(func(v uintptr) uintptr { return v })
		rev := cmpx.New[uintptr]().Rev(func(v uintptr) uintptr { return v })
		vals := []uintptr{0, 1, 255, 256, math.MaxUint64 - 1, math.MaxUint64}
		ascending := make([]uintptr, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := uintptr(rng.Uint64()), uintptr(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("uintptr named", func(t *testing.T) {
		type named uintptr
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []uintptr{0, 1, 255, 256, math.MaxUint64 - 1, math.MaxUint64}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(uintptr(rng.Uint64())), named(uintptr(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("float64", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[float64]().Cmp(func(v float64) float64 { return v })
		rev := cmpx.New[float64]().Rev(func(v float64) float64 { return v })
		vals := []float64{math.Inf(-1), -math.MaxFloat64, -1.5, -1, -math.SmallestNonzeroFloat64, 0, math.SmallestNonzeroFloat64, 1, 1.5, math.MaxFloat64, math.Inf(1)}
		ascending := make([]float64, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := math.Float64frombits(rng.Uint64()), math.Float64frombits(rng.Uint64())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("float64 named", func(t *testing.T) {
		type named float64
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []float64{math.Inf(-1), -math.MaxFloat64, -1.5, -1, -math.SmallestNonzeroFloat64, 0, math.SmallestNonzeroFloat64, 1, 1.5, math.MaxFloat64, math.Inf(1)}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(math.Float64frombits(rng.Uint64())), named(math.Float64frombits(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("float32", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[float32]().Cmp(func(v float32) float32 { return v })
		rev := cmpx.New[float32]().Rev(func(v float32) float32 { return v })
		vals := []float32{float32(math.Inf(-1)), -math.MaxFloat32, -1.5, -1, -math.SmallestNonzeroFloat32, 0, math.SmallestNonzeroFloat32, 1, 1.5, 16777216, 16777218, math.MaxFloat32, float32(math.Inf(1))}
		ascending := make([]float32, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := math.Float32frombits(uint32(rng.Uint64())), math.Float32frombits(uint32(rng.Uint64()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("float32 named", func(t *testing.T) {
		type named float32
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []float32{float32(math.Inf(-1)), -math.MaxFloat32, -1.5, -1, -math.SmallestNonzeroFloat32, 0, math.SmallestNonzeroFloat32, 1, 1.5, 16777216, 16777218, math.MaxFloat32, float32(math.Inf(1))}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(math.Float32frombits(uint32(rng.Uint64()))), named(math.Float32frombits(uint32(rng.Uint64())))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("string", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[string]().Cmp(func(v string) string { return v })
		rev := cmpx.New[string]().Rev(func(v string) string { return v })
		vals := []string{"", "\x00", "\x00a", "\x01", "a", "a\x00", "a\x00\x01", "a\x00b", "ab", "b", "é", "é\x00", "\xff"}
		ascending := make([]string, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := string(func() []byte {
				b := make([]byte, rng.IntN(5))
				for i := range b {
					b[i] = []byte{0, 1, 'a', 'b', 0xff}[rng.IntN(5)]
				}
				return b
			}()), string(func() []byte {
				b := make([]byte, rng.IntN(5))
				for i := range b {
					b[i] = []byte{0, 1, 'a', 'b', 0xff}[rng.IntN(5)]
				}
				return b
			}())
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("string named", func(t *testing.T) {
		type named string
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []string{"", "\x00", "\x00a", "\x01", "a", "a\x00", "a\x00\x01", "a\x00b", "ab", "b", "é", "é\x00", "\xff"}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(string(func() []byte {
				b := make([]byte, rng.IntN(5))
				for i := range b {
					b[i] = []byte{0, 1, 'a', 'b', 0xff}[rng.IntN(5)]
				}
				return b
			}())), named(string(func() []byte {
				b := make([]byte, rng.IntN(5))
				for i := range b {
					b[i] = []byte{0, 1, 'a', 'b', 0xff}[rng.IntN(5)]
				}
				return b
			}()))
			want := cmp.Compare(a, b)
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("bool", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[bool]().Cmp(func(v bool) bool { return v })
		rev := cmpx.New[bool]().Rev(func(v bool) bool { return v })
		vals := []bool{false, true}
		ascending := make([]bool, len(vals))
		for i := range vals {
			ascending[i] = vals[i]
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := rng.IntN(2) == 0, rng.IntN(2) == 0
			want := func() int {
				if a == b {
					return 0
				}
				if !a {
					return -1
				}
				return 1
			}()
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("bool named", func(t *testing.T) {
		type named bool
		rng := rand.New(rand.NewPCG(1, 2))
		order := cmpx.New[named]().Cmp(func(v named) named { return v })
		rev := cmpx.New[named]().Rev(func(v named) named { return v })
		vals := []bool{false, true}
		ascending := make([]named, len(vals))
		for i := range vals {
			ascending[i] = named(vals[i])
		}

		shuffled := slices.Clone(ascending)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		slices.SortStableFunc(shuffled, order.Compare)
		require.Equal(t, ascending, shuffled)

		for i := range ascending {
			for j := range ascending {
				require.Equal(t, cmp.Compare(i, j), order.Compare(ascending[i], ascending[j]))
				require.Equal(t, cmp.Compare(j, i), rev.Compare(ascending[i], ascending[j]))
			}
		}

		descending := slices.Clone(ascending)
		slices.Reverse(descending)
		slices.SortStableFunc(shuffled, rev.Compare)
		require.Equal(t, descending, shuffled)

		for range 3000 {
			a, b := named(rng.IntN(2) == 0), named(rng.IntN(2) == 0)
			want := func() int {
				if a == b {
					return 0
				}
				if !a {
					return -1
				}
				return 1
			}()
			require.Equal(t, want, order.Compare(a, b))
			require.Equal(t, -want, order.Compare(b, a))
			require.Equal(t, -want, rev.Compare(a, b))
		}
	})

	t.Run("float64 specials", func(t *testing.T) {
		order := cmpx.New[float64]().Cmp(func(v float64) float64 { return v })

		require.Equal(t, 0, order.Compare(math.Copysign(0, -1), 0))
		require.Equal(t, 0, order.Compare(math.NaN(), math.NaN()))
		require.Equal(t, -1, order.Compare(math.NaN(), math.Inf(-1)))
		require.Equal(t, -1, order.Compare(math.NaN(), -math.MaxFloat64))
		require.Equal(t, 1, order.Compare(math.Inf(-1), math.NaN()))
		require.Equal(t, 0, order.Compare(math.Float64frombits(0x7ff8000000000001), math.Float64frombits(0xfff8000000000002)))
	})

	t.Run("float32 specials", func(t *testing.T) {
		order := cmpx.New[float32]().Cmp(func(v float32) float32 { return v })

		require.Equal(t, 0, order.Compare(float32(math.Copysign(0, -1)), 0))
		require.Equal(t, 0, order.Compare(float32(math.NaN()), float32(math.NaN())))
		require.Equal(t, -1, order.Compare(float32(math.NaN()), float32(math.Inf(-1))))
		// 16777217 is not representable as a float32, it rounds to 16777216.
		require.Equal(t, 0, order.Compare(16777217, 16777216))
	})

	t.Run("string terminator cannot be forged", func(t *testing.T) {
		order := cmpx.New[string]().Cmp(func(v string) string { return v })

		require.Equal(t, 1, order.Compare("a\x00\x01", "a"))
		require.Equal(t, -1, order.Compare("a", "a\x00\x01"))
		require.Equal(t, -1, order.Compare(strings.Repeat("x", 1000)+"a", strings.Repeat("x", 1000)+"b"))
		require.Equal(t, -1, order.Compare(strings.Repeat("x", 1000), strings.Repeat("x", 1001)))
	})
}

func TestRev(t *testing.T) {
	type row struct {
		name   string
		weight int
		ok     bool
	}

	t.Run("should keep the position of the key and invert only its own order", func(t *testing.T) {
		rows := []row{{"a", 1, false}, {"ab", 2, false}, {"a", 3, true}, {"b", 1, false}, {"ab", 1, true}, {"a", 2, false}}

		slices.SortStableFunc(rows, cmpx.New[row]().Rev(func(v row) string { return v.name }).Cmp(func(v row) int { return v.weight }).Compare)
		require.Equal(t, []row{{"b", 1, false}, {"ab", 1, true}, {"ab", 2, false}, {"a", 1, false}, {"a", 2, false}, {"a", 3, true}}, rows)

		slices.SortStableFunc(rows, cmpx.New[row]().Cmp(func(v row) string { return v.name }).Rev(func(v row) int { return v.weight }).Compare)
		require.Equal(t, []row{{"a", 3, true}, {"a", 2, false}, {"a", 1, false}, {"ab", 2, false}, {"ab", 1, true}, {"b", 1, false}}, rows)
	})

	t.Run("should put true first for a bool key", func(t *testing.T) {
		rows := []row{{"a", 1, false}, {"b", 2, true}, {"c", 3, false}, {"d", 4, true}}

		slices.SortStableFunc(rows, cmpx.New[row]().Rev(func(v row) bool { return v.ok }).Cmp(func(v row) string { return v.name }).Compare)
		require.Equal(t, []row{{"b", 2, true}, {"d", 4, true}, {"a", 1, false}, {"c", 3, false}}, rows)
	})

	t.Run("should not bleed an inverted string into the next key", func(t *testing.T) {
		order := cmpx.New[row]().Rev(func(v row) string { return v.name }).Cmp(func(v row) int { return v.weight })

		require.Equal(t, 1, order.Compare(row{name: "a", weight: 2}, row{name: "ab", weight: 1}))
		require.Equal(t, -1, order.Compare(row{name: "ab", weight: 1}, row{name: "a", weight: 2}))
		require.Equal(t, -1, order.Compare(row{name: "a\x00", weight: 1}, row{name: "a", weight: 1}))
		require.Equal(t, -1, order.Compare(row{name: "a", weight: 1}, row{name: "a", weight: 2}))
		require.Equal(t, 0, order.Compare(row{name: "a", weight: 1}, row{name: "a", weight: 1}))
	})

	t.Run("should invert floats like cmp.Compare reversed", func(t *testing.T) {
		order := cmpx.New[float64]().Rev(func(v float64) float64 { return v })

		require.Equal(t, 1, order.Compare(math.NaN(), math.Inf(-1)))
		require.Equal(t, 0, order.Compare(math.Copysign(0, -1), 0))
		require.Equal(t, -1, order.Compare(math.Inf(1), 1))
	})
}

func TestBytes(t *testing.T) {
	order := cmpx.New[[]byte]().Bytes(func(v []byte) []byte { return v })

	require.Equal(t, -1, order.Compare([]byte("a"), []byte("ab")))
	require.Equal(t, -1, order.Compare([]byte("a"), []byte("a\x00")))
	require.Equal(t, 1, order.Compare([]byte("b"), []byte("a")))
	require.Equal(t, 0, order.Compare(nil, []byte{}))
}

func TestTime(t *testing.T) {
	order := cmpx.New[time.Time]().Time(func(v time.Time) time.Time { return v })
	est := time.FixedZone("est", -5*3600)

	v := []time.Time{
		time.Unix(10, 5),
		time.Unix(0, 0),
		time.Unix(-10, 0),
		time.Unix(10, 1),
	}
	slices.SortStableFunc(v, order.Compare)
	require.Equal(t, []time.Time{time.Unix(-10, 0), time.Unix(0, 0), time.Unix(10, 1), time.Unix(10, 5)}, v)

	require.Equal(t, 0, order.Compare(time.Unix(100, 7), time.Unix(100, 7).In(est)))
}

func TestMixedKeys(t *testing.T) {
	type row struct {
		current bool
		weight  int
		ratio   float64
		at      time.Time
		name    string
	}

	rows := []row{
		{true, 1, 0.5, time.Unix(5, 0), "a"},
		{false, 2, 0.5, time.Unix(1, 0), "b"},
		{false, 2, 0.5, time.Unix(1, 0), "a"},
		{false, -1, 9, time.Unix(9, 0), "z"},
		{false, 2, -3, time.Unix(0, 0), "q"},
		{false, 2, 0.5, time.Unix(1, 0), "a\x00"},
	}

	order := cmpx.New[row]().
		Cmp(func(v row) bool { return !v.current }).
		Cmp(func(v row) int { return v.weight }).
		Cmp(func(v row) float64 { return v.ratio }).
		Time(func(v row) time.Time { return v.at }).
		Cmp(func(v row) string { return v.name })
	slices.SortStableFunc(rows, order.Compare)

	require.Equal(t, []row{
		{true, 1, 0.5, time.Unix(5, 0), "a"},
		{false, -1, 9, time.Unix(9, 0), "z"},
		{false, 2, -3, time.Unix(0, 0), "q"},
		{false, 2, 0.5, time.Unix(1, 0), "a"},
		{false, 2, 0.5, time.Unix(1, 0), "a\x00"},
		{false, 2, 0.5, time.Unix(1, 0), "b"},
	}, rows)
}

func TestEmptyComparisonIsEqual(t *testing.T) {
	require.Equal(t, 0, cmpx.New[int]().Compare(1, 2))

	var zero cmpx.C[int]
	require.Equal(t, 0, zero.Compare(1, 2))
}

func TestKeysAreNotShared(t *testing.T) {
	base := cmpx.New[string]()
	first := base.Cmp(func(v string) string { return v })
	_ = base.Cmp(func(v string) string { return "x" })

	require.Equal(t, -1, first.Compare("a", "b"))
	require.Equal(t, 0, base.Compare("a", "b"))
}

func TestFromWith(t *testing.T) {
	order := cmpx.From("b").Cmp(func(v string) string { return v })

	require.Equal(t, -1, order.With("c"))
	require.Equal(t, 0, order.With("b"))
	require.Equal(t, 1, order.With("a"))
}
