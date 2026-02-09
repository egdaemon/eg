package egmatrix_test

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/egdaemon/eg/runtime/x/wasi/egmatrix"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Field1  string
	Field2  bool
	Field3  int64
	Field4  float64
	Int     int
	Int8    int8
	Int16   int16
	Int32   int32
	Uint    uint
	Uint8   uint8
	Uint16  uint16
	Uint32  uint32
	Uint64  uint64
	Float32 float32
	Dur     time.Duration
	Time    time.Time
	Addr    netip.Addr
	Prefix  netip.Prefix
}

func TestBuilder(t *testing.T) {
	t.Run("New creates builder", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		require.NotNil(t, m)
	})

	t.Run("Assign returns builder for chaining", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		result := egmatrix.Assign(m, func(dst *testStruct, v string) { dst.Field1 = v }, "a", "b", "c")
		require.NotNil(t, result)
		require.Equal(t, m, result)
	})

	t.Run("Boolean generates 2 permutations", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Boolean(func(dst *testStruct, v bool) { dst.Field2 = v })

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 2)
		require.True(t, results[0].Field2)
		require.False(t, results[1].Field2)
	})

	t.Run("String generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]().
			String(func(dst *testStruct, v string) { dst.Field1 = v }, "a", "b", "c")

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, "a", results[0].Field1)
		require.Equal(t, "b", results[1].Field1)
		require.Equal(t, "c", results[2].Field1)
	})

	t.Run("Int64 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Int64(func(dst *testStruct, v int64) { dst.Field3 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, int64(1), results[0].Field3)
		require.Equal(t, int64(2), results[1].Field3)
		require.Equal(t, int64(3), results[2].Field3)
	})

	t.Run("Int generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Int(func(dst *testStruct, v int) { dst.Int = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, 1, results[0].Int)
		require.Equal(t, 2, results[1].Int)
		require.Equal(t, 3, results[2].Int)
	})

	t.Run("Int8 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Int8(func(dst *testStruct, v int8) { dst.Int8 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, int8(1), results[0].Int8)
		require.Equal(t, int8(2), results[1].Int8)
		require.Equal(t, int8(3), results[2].Int8)
	})

	t.Run("Int16 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Int16(func(dst *testStruct, v int16) { dst.Int16 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, int16(1), results[0].Int16)
		require.Equal(t, int16(2), results[1].Int16)
		require.Equal(t, int16(3), results[2].Int16)
	})

	t.Run("Int32 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Int32(func(dst *testStruct, v int32) { dst.Int32 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, int32(1), results[0].Int32)
		require.Equal(t, int32(2), results[1].Int32)
		require.Equal(t, int32(3), results[2].Int32)
	})

	t.Run("Uint generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Uint(func(dst *testStruct, v uint) { dst.Uint = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, uint(1), results[0].Uint)
		require.Equal(t, uint(2), results[1].Uint)
		require.Equal(t, uint(3), results[2].Uint)
	})

	t.Run("Uint8 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Uint8(func(dst *testStruct, v uint8) { dst.Uint8 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, uint8(1), results[0].Uint8)
		require.Equal(t, uint8(2), results[1].Uint8)
		require.Equal(t, uint8(3), results[2].Uint8)
	})

	t.Run("Uint16 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Uint16(func(dst *testStruct, v uint16) { dst.Uint16 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, uint16(1), results[0].Uint16)
		require.Equal(t, uint16(2), results[1].Uint16)
		require.Equal(t, uint16(3), results[2].Uint16)
	})

	t.Run("Uint32 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Uint32(func(dst *testStruct, v uint32) { dst.Uint32 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, uint32(1), results[0].Uint32)
		require.Equal(t, uint32(2), results[1].Uint32)
		require.Equal(t, uint32(3), results[2].Uint32)
	})

	t.Run("Uint64 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Uint64(func(dst *testStruct, v uint64) { dst.Uint64 = v }, 1, 2, 3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, uint64(1), results[0].Uint64)
		require.Equal(t, uint64(2), results[1].Uint64)
		require.Equal(t, uint64(3), results[2].Uint64)
	})

	t.Run("Float32 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Float32(func(dst *testStruct, v float32) { dst.Float32 = v }, 1.1, 2.2, 3.3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.EqualValues(t, float32(1.1), results[0].Float32)
		require.EqualValues(t, float32(2.2), results[1].Float32)
		require.EqualValues(t, float32(3.3), results[2].Float32)
	})

	t.Run("Float64 generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Float64(func(dst *testStruct, v float64) { dst.Field4 = v }, 1.1, 2.2, 3.3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.EqualValues(t, 1.1, results[0].Field4)
		require.EqualValues(t, 2.2, results[1].Field4)
		require.EqualValues(t, 3.3, results[2].Field4)
	})

	t.Run("Duration generates permutations for each option", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Duration(func(dst *testStruct, v time.Duration) { dst.Dur = v }, time.Second, time.Minute, time.Hour)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, time.Second, results[0].Dur)
		require.Equal(t, time.Minute, results[1].Dur)
		require.Equal(t, time.Hour, results[2].Dur)
	})

	t.Run("Time generates permutations for each option", func(t *testing.T) {
		t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

		m := egmatrix.New[testStruct]()
		m.Time(func(dst *testStruct, v time.Time) { dst.Time = v }, t1, t2)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 2)
		require.Equal(t, t1, results[0].Time)
		require.Equal(t, t2, results[1].Time)
	})

	t.Run("Addr generates permutations for each option", func(t *testing.T) {
		a1 := netip.MustParseAddr("192.168.1.1")
		a2 := netip.MustParseAddr("10.0.0.1")
		a3 := netip.MustParseAddr("::1")

		m := egmatrix.New[testStruct]()
		m.Addr(func(dst *testStruct, v netip.Addr) { dst.Addr = v }, a1, a2, a3)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, a1, results[0].Addr)
		require.Equal(t, a2, results[1].Addr)
		require.Equal(t, a3, results[2].Addr)
	})

	t.Run("Prefix generates permutations for each option", func(t *testing.T) {
		p1 := netip.MustParsePrefix("192.168.1.0/24")
		p2 := netip.MustParsePrefix("10.0.0.0/8")

		m := egmatrix.New[testStruct]()
		m.Prefix(func(dst *testStruct, v netip.Prefix) { dst.Prefix = v }, p1, p2)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 2)
		require.Equal(t, p1, results[0].Prefix)
		require.Equal(t, p2, results[1].Prefix)
	})

	t.Run("method chaining", func(t *testing.T) {
		m := egmatrix.New[testStruct]().
			Boolean(func(dst *testStruct, v bool) { dst.Field2 = v }).
			String(func(dst *testStruct, v string) { dst.Field1 = v }, "a", "b")

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 4)
	})
}

func TestPerm(t *testing.T) {
	t.Run("cartesian product of multiple fields", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Boolean(func(dst *testStruct, v bool) { dst.Field2 = v })
		m.String(func(dst *testStruct, v string) { dst.Field1 = v }, "a", "b")

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 4)

		expected := []testStruct{
			{Field1: "a", Field2: true},
			{Field1: "b", Field2: true},
			{Field1: "a", Field2: false},
			{Field1: "b", Field2: false},
		}

		for _, exp := range expected {
			require.True(t, slices.Contains(results, exp), "should contain permutation %+v", exp)
		}
	})

	t.Run("cartesian product of three fields", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.Boolean(func(dst *testStruct, v bool) { dst.Field2 = v })
		m.String(func(dst *testStruct, v string) { dst.Field1 = v }, "x", "y")
		m.Int64(func(dst *testStruct, v int64) { dst.Field3 = v }, 10, 20)

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 8)
	})

	t.Run("empty builder yields nothing", func(t *testing.T) {
		m := egmatrix.New[testStruct]()

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 0)
	})

	t.Run("single option yields one permutation", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.String(func(dst *testStruct, v string) { dst.Field1 = v }, "single")

		var results []testStruct
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 1)
		require.Equal(t, "single", results[0].Field1)
	})

	t.Run("early termination", func(t *testing.T) {
		m := egmatrix.New[testStruct]()
		m.String(func(dst *testStruct, v string) { dst.Field1 = v }, "a", "b", "c", "d", "e")

		count := 0
		for range m.Perm() {
			count++
			if count >= 3 {
				break
			}
		}

		require.Equal(t, 3, count)
	})
}

func TestCustomTypes(t *testing.T) {
	type customType struct {
		Value int
	}

	type structWithCustom struct {
		Custom customType
	}

	t.Run("Assign works with custom types", func(t *testing.T) {
		m := egmatrix.New[structWithCustom]()
		egmatrix.Assign(
			m,
			func(dst *structWithCustom, v customType) { dst.Custom = v },
			customType{Value: 1},
			customType{Value: 2},
			customType{Value: 3},
		)

		var results []structWithCustom
		for v := range m.Perm() {
			results = append(results, v)
		}

		require.Len(t, results, 3)
		require.Equal(t, 1, results[0].Custom.Value)
		require.Equal(t, 2, results[1].Custom.Value)
		require.Equal(t, 3, results[2].Custom.Value)
	})
}
