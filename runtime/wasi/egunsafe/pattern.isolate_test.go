package egunsafe_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestPatternIsolateFrom(t *testing.T) {
	id := uuid.Must(uuid.FromString("01890a5d-ac96-774b-bcce-b302099a8057"))

	t.Run("substitutes the last n bytes as hex", func(t *testing.T) {
		require.Equal(t, "m.099a8057.d", egunsafe.PatternIsolateFrom(id, 4, "m.%x.d"))
	})

	t.Run("substitutes the last n bytes as url safe base64", func(t *testing.T) {
		require.Equal(t, "m.CZqAVw.d", egunsafe.PatternIsolateFrom(id, 4, "m.%s.d"))
	})

	t.Run("base64 uses the url safe alphabet without padding", func(t *testing.T) {
		require.Equal(t, "AYkKXayWd0u8zrMCCZqAVw", egunsafe.PatternIsolateFrom(id, uuid.Size, "%s"))
	})

	t.Run("precision 16 encodes as hex", func(t *testing.T) {
		require.Equal(t, "m.099a8057.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.16s.d"))
	})

	t.Run("precision 32 encodes as lowercase base32", func(t *testing.T) {
		require.Equal(t, "m.16d80lo.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.32s.d"))
	})

	t.Run("precision 64 encodes as url safe base64", func(t *testing.T) {
		require.Equal(t, "m.CZqAVw.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.64s.d"))
	})

	t.Run("unsupported precision defaults to url safe base64", func(t *testing.T) {
		require.Equal(t, "m.CZqAVw.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.8s.d"))
	})

	t.Run("hex precision 16 encodes as hex", func(t *testing.T) {
		require.Equal(t, "m.099a8057.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.16x.d"))
	})

	t.Run("hex precision 32 encodes as lowercase base32", func(t *testing.T) {
		require.Equal(t, "m.16d80lo.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.32x.d"))
	})

	t.Run("hex precision 64 encodes as url safe base64", func(t *testing.T) {
		require.Equal(t, "m.CZqAVw.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.64x.d"))
	})

	t.Run("hex unsupported precision defaults to hex", func(t *testing.T) {
		require.Equal(t, "m.099a8057.d", egunsafe.PatternIsolateFrom(id, 4, "m.%.8x.d"))
	})

	t.Run("other verbs default to url safe base64", func(t *testing.T) {
		require.Equal(t, "CZqAVw", egunsafe.PatternIsolateFrom(id, 4, "%v"))
	})

	t.Run("other verbs honor precision", func(t *testing.T) {
		require.Equal(t, "16d80lo", egunsafe.PatternIsolateFrom(id, 4, "%.32v"))
	})

	t.Run("zero length substitutes nothing", func(t *testing.T) {
		require.Equal(t, "xy", egunsafe.PatternIsolateFrom(id, 0, "x%xy"))
	})

	t.Run("full length substitutes the entire id", func(t *testing.T) {
		require.Equal(t, "01890a5dac96774bbcceb302099a8057", egunsafe.PatternIsolateFrom(id, uuid.Size, "%x"))
	})
}

func TestPatternIsolate(t *testing.T) {
	t.Run("substitutes n random bytes as hex", func(t *testing.T) {
		require.Regexp(t, `^p\.[0-9a-f]{8}\.s$`, egunsafe.PatternIsolate(4, "p.%x.s"))
	})

	t.Run("substitutes n random bytes as url safe base64", func(t *testing.T) {
		require.Regexp(t, `^p\.[A-Za-z0-9_-]{6}\.s$`, egunsafe.PatternIsolate(4, "p.%s.s"))
	})

	t.Run("generates distinct values", func(t *testing.T) {
		require.NotEqual(t, egunsafe.PatternIsolate(4, "p.%x.s"), egunsafe.PatternIsolate(4, "p.%x.s"))
	})
}
