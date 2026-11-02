package cmpx_test

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/egdaemon/eg/internal/cmpx"
	"github.com/stretchr/testify/require"
)

type nint int
type nstring string
type nfloat float64
type nbool bool

type deep struct {
	flag bool
	u    uint16
}

type inner struct {
	n    int64
	deep deep
}

// the field order is deliberate, the narrow fields in front of wide ones exercise padding.
type byrow struct {
	id     int
	b      bool
	i8     int8
	i      int
	i16    int16
	i32    int32
	i64    int64
	u8     uint8
	u      uint
	u16    uint16
	u32    uint32
	u64    uint64
	up     uintptr
	f32    float32
	f64    float64
	s      string
	nested inner
	ni     nint
	ns     nstring
	nf     nfloat
	nb     nbool
	d      time.Duration
	arr    [3]int8
}

type withpointer struct {
	n *inner
	i int
}

func TestBy(t *testing.T) {

	t.Run("b", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].b = rng.IntN(2) == 0
			} else {
				rows[i].b = rng.IntN(2) == 0
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) bool { return v.b }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.b).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.b)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("i8", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].i8 = int8(rng.IntN(4) - 2)
			} else {
				rows[i].i8 = int8(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int8 { return v.i8 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.i8).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.i8)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("i", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].i = rng.IntN(4) - 2
			} else {
				rows[i].i = int(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int { return v.i }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.i).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.i)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("i16", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].i16 = int16(rng.IntN(4) - 2)
			} else {
				rows[i].i16 = int16(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int16 { return v.i16 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.i16).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.i16)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("i32", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].i32 = int32(rng.IntN(4) - 2)
			} else {
				rows[i].i32 = int32(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int32 { return v.i32 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.i32).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.i32)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("i64", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].i64 = int64(rng.IntN(4) - 2)
			} else {
				rows[i].i64 = int64(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int64 { return v.i64 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.i64).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.i64)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("u8", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].u8 = uint8(rng.IntN(4))
			} else {
				rows[i].u8 = uint8(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uint8 { return v.u8 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.u8).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.u8)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("u", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].u = uint(rng.IntN(4))
			} else {
				rows[i].u = uint(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uint { return v.u }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.u).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.u)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("u16", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].u16 = uint16(rng.IntN(4))
			} else {
				rows[i].u16 = uint16(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uint16 { return v.u16 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.u16).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.u16)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("u32", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].u32 = uint32(rng.IntN(4))
			} else {
				rows[i].u32 = uint32(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uint32 { return v.u32 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.u32).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.u32)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("u64", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].u64 = uint64(rng.IntN(4))
			} else {
				rows[i].u64 = rng.Uint64()
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uint64 { return v.u64 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.u64).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.u64)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("up", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].up = uintptr(rng.IntN(4))
			} else {
				rows[i].up = uintptr(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uintptr { return v.up }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.up).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.up)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("f32", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].f32 = float32(rng.IntN(4) - 2)
			} else {
				rows[i].f32 = math.Float32frombits(uint32(rng.Uint64()))
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) float32 { return v.f32 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.f32).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.f32)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("f64", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].f64 = float64(rng.IntN(4) - 2)
			} else {
				rows[i].f64 = math.Float64frombits(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) float64 { return v.f64 }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.f64).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.f64)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("s", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].s = []string{"", "a", "a\x00", "b"}[rng.IntN(4)]
			} else {
				rows[i].s = string([]byte{byte(rng.Uint64()), byte(rng.Uint64())})
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) string { return v.s }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.s).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.s)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("ni", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].ni = nint(rng.IntN(4) - 2)
			} else {
				rows[i].ni = nint(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) nint { return v.ni }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.ni).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.ni)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("ns", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].ns = nstring([]string{"", "a", "a\x00", "b"}[rng.IntN(4)])
			} else {
				rows[i].ns = nstring([]byte{byte(rng.Uint64()), byte(rng.Uint64())})
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) nstring { return v.ns }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.ns).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.ns)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("nf", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].nf = nfloat(rng.IntN(4) - 2)
			} else {
				rows[i].nf = nfloat(math.Float64frombits(rng.Uint64()))
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) nfloat { return v.nf }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.nf).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.nf)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("nb", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].nb = rng.IntN(2) == 0
			} else {
				rows[i].nb = rng.IntN(2) == 0
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) nbool { return v.nb }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.nb).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.nb)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("d", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].d = time.Duration(rng.IntN(4) - 2)
			} else {
				rows[i].d = time.Duration(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) time.Duration { return v.d }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.d).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.d)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("nested.n", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].nested.n = int64(rng.IntN(4) - 2)
			} else {
				rows[i].nested.n = int64(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int64 { return v.nested.n }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.nested.n).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.nested.n)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("nested.deep.u", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].nested.deep.u = uint16(rng.IntN(4))
			} else {
				rows[i].nested.deep.u = uint16(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) uint16 { return v.nested.deep.u }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.nested.deep.u).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.nested.deep.u)
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("arr[1]", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			if rng.IntN(2) == 0 {
				rows[i].arr[1] = int8(rng.IntN(4) - 2)
			} else {
				rows[i].arr[1] = int8(rng.Uint64())
			}
		}

		// the Cmp based ordering is the reference.
		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().Cmp(func(v byrow) int8 { return v.arr[1] }).Compare)

		var z byrow

		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.arr[1]).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		ptrs := make([]*byrow, 0, len(rows)+1)
		for i := range rows {
			ptrs = append(ptrs, &rows[i])
		}
		ptrs = slices.Insert(ptrs, 3, nil)
		ptrorder := cmpx.From(&z).By(&z.arr[1])
		slices.SortStableFunc(ptrs, ptrorder.Compare)
		require.Nil(t, ptrs[0])
		for i := range want {
			require.Equal(t, want[i].id, ptrs[i+1].id)
		}
		require.Equal(t, 0, ptrorder.Compare(nil, nil))
		require.Equal(t, -1, ptrorder.Compare(nil, &rows[0]))
		require.Equal(t, 1, ptrorder.Compare(&rows[0], nil))
	})

	t.Run("chained keys break ties with the next key", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(3, 4))
		rows := make([]byrow, 400)
		for i := range rows {
			rows[i].id = i
			rows[i].i8 = int8(rng.IntN(3))
			rows[i].s = []string{"", "a", "a\x00", "b"}[rng.IntN(4)]
			rows[i].nested.n = int64(rng.IntN(3) - 1)
		}

		want := slices.Clone(rows)
		slices.SortStableFunc(want, cmpx.New[byrow]().
			Cmp(func(v byrow) int8 { return v.i8 }).
			Cmp(func(v byrow) string { return v.s }).
			Cmp(func(v byrow) int64 { return v.nested.n }).Compare)

		var z byrow
		got := slices.Clone(rows)
		slices.SortStableFunc(got, cmpx.For(&z).By(&z.i8).By(&z.s).By(&z.nested.n).Compare)
		for i := range want {
			require.Equal(t, want[i].id, got[i].id)
		}

		require.Equal(t, -1, cmpx.For(&z).By(&z.i8).By(&z.s).Compare(byrow{i8: 1, s: "a"}, byrow{i8: 1, s: "b"}))
		require.Equal(t, 1, cmpx.For(&z).By(&z.i8).By(&z.s).Compare(byrow{i8: 2, s: ""}, byrow{i8: 1, s: "b"}))
	})

	t.Run("by can be mixed with cmp and time keys", func(t *testing.T) {
		var z byrow

		order := cmpx.For(&z).By(&z.i).Cmp(func(v byrow) string { return v.s }).By(&z.f64)
		require.Equal(t, -1, order.Compare(byrow{i: 1, s: "a", f64: 9}, byrow{i: 1, s: "b", f64: 1}))
		require.Equal(t, -1, order.Compare(byrow{i: 1, s: "a", f64: 1}, byrow{i: 1, s: "a", f64: 9}))
		require.Equal(t, 0, order.Compare(byrow{i: 1, s: "a", f64: 1}, byrow{i: 1, s: "a", f64: 1}))
	})

	t.Run("with compares against the sample pointer", func(t *testing.T) {
		a, b := &byrow{i: 1}, &byrow{i: 2}

		require.Equal(t, -1, cmpx.From(a).By(&a.i).With(b))
		require.Equal(t, 1, cmpx.From(b).By(&b.i).With(a))
		require.Equal(t, 0, cmpx.From(a).By(&a.i).With(a))
	})

	t.Run("should panic without a sample", func(t *testing.T) {
		var z byrow

		// a C has no By, it does not compile without For or From.
		require.Panics(t, func() { cmpx.From(byrow{}).By(&z.i) })
		require.Panics(t, func() { cmpx.From((*byrow)(nil)).By(&z.i) })
		require.Panics(t, func() { cmpx.From(&z.i).By(&z.i) })
		require.Panics(t, func() { cmpx.From("a").By(&z.s) })
	})

	t.Run("should panic when the sample is not a struct or is nil", func(t *testing.T) {
		require.Panics(t, func() { cmpx.For(new(int)) })
		require.Panics(t, func() { cmpx.For[byrow](nil) })
	})

	t.Run("should panic when the field is not part of the sample", func(t *testing.T) {
		var z, other byrow

		require.Panics(t, func() { cmpx.For(&z).By(&other.i) })
		require.Panics(t, func() { cmpx.From(&z).By(&other.s) })
		local := 1
		require.Panics(t, func() { cmpx.For(&z).By(&local) })
	})

	t.Run("should panic when the field is reached through a pointer", func(t *testing.T) {
		z := withpointer{n: &inner{}}

		require.NotPanics(t, func() { cmpx.For(&z).By(&z.i) })
		require.Panics(t, func() { cmpx.For(&z).By(&z.n.n) })
	})

	t.Run("should panic when the type does not match the field", func(t *testing.T) {
		var z byrow

		require.Panics(t, func() { cmpx.For(&z).By((*int64)(unsafe.Pointer(&z.i32))) })
		require.Panics(t, func() { cmpx.For(&z).By((*uint8)(unsafe.Pointer(&z.i8))) })
		require.Panics(t, func() { cmpx.For(&z).By((*int)(unsafe.Pointer(&z.ni))) })
		require.Panics(t, func() { cmpx.For(&z).By((*int8)(unsafe.Pointer(&z.nested))) })
		require.Panics(t, func() { cmpx.For(&z).By((*int64)(unsafe.Pointer(&z.arr))) })
	})
}
