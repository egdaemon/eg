package egautogentest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolang(t *testing.T) {
	t.Run("binds the sequence", func(t *testing.T) {
		seq := From(Fn{Name: "Greet"})
		require.Equal(t, seq, Golang(seq).Seq())
	})

	t.Run("applies style", func(t *testing.T) {
		require.Equal(t, "s", Golang(From()).Style("s").resolve().style)
	})

	t.Run("later options win", func(t *testing.T) {
		require.Equal(t, "b", Golang(From()).Style("a").Style("b").resolve().style)
	})

	t.Run("chaining does not modify the receiver", func(t *testing.T) {
		base := Golang(From()).Style("a")
		base.Style("b")
		require.Equal(t, "a", base.resolve().style)
	})
}
