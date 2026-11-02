package cmpx_test

import (
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/cmpx"
	"github.com/stretchr/testify/require"
)

func TestBool(t *testing.T) {
	odd := func(v int) bool { return v%2 == 1 }

	require.Equal(t, -1, cmpx.From(1).Bool(odd).With(2))
	require.Equal(t, 1, cmpx.From(2).Bool(odd).With(1))
	require.Equal(t, 0, cmpx.From(1).Bool(odd).With(3))
}

func TestString(t *testing.T) {
	id := func(v string) string { return v }

	require.Equal(t, -1, cmpx.From("a").String(id).With("b"))
	require.Equal(t, 1, cmpx.From("b").String(id).With("a"))
	require.Equal(t, 0, cmpx.From("a").String(id).With("a"))
	// a prefix orders before its extensions.
	require.Equal(t, -1, cmpx.From("a").String(id).With("ab"))
	require.Equal(t, -1, cmpx.From("a\x00").String(id).With("a\x00b"))
	require.Equal(t, -1, cmpx.From("a").String(id).With("a\x00"))
}

func TestChained(t *testing.T) {
	type row struct {
		current bool
		id      string
	}

	rows := []row{{false, "a"}, {true, "c"}, {false, "b"}, {true, "a"}, {true, "ab"}}

	slices.SortStableFunc(rows, func(a, b row) int {
		return cmpx.From(a).Bool(func(v row) bool { return v.current }).String(func(v row) string { return v.id }).With(b)
	})

	require.Equal(t, []row{{true, "a"}, {true, "ab"}, {true, "c"}, {false, "a"}, {false, "b"}}, rows)
}

func TestFromIsNotMutated(t *testing.T) {
	base := cmpx.From("a")
	first := base.String(func(v string) string { return v })
	_ = base.String(func(v string) string { return "x" })

	require.Equal(t, -1, first.With("b"))
}

func TestFor(t *testing.T) {
	type row struct {
		current bool
		id      string
	}

	rows := []row{{false, "a"}, {true, "c"}, {false, "b"}, {true, "a"}}

	order := cmpx.For[row]().Bool(func(v row) bool { return v.current }).String(func(v row) string { return v.id })
	slices.SortStableFunc(rows, order.Compare)

	require.Equal(t, []row{{true, "a"}, {true, "c"}, {false, "a"}, {false, "b"}}, rows)
	require.Equal(t, 0, cmpx.For[row]().Compare(row{true, "a"}, row{false, "b"}))
}
