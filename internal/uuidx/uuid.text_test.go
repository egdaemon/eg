package uuidx_test

import (
	"encoding/json"
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestMarshalText(t *testing.T) {
	t.Run("should produce the canonical lowercase form", func(t *testing.T) {
		b, err := MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00").MarshalText()
		require.NoError(t, err)
		require.Equal(t, []byte("11223344-5566-7788-99aa-bbccddeeff00"), b)
	})

	t.Run("should encode as a json string", func(t *testing.T) {
		b, err := json.Marshal(struct{ ID UUID }{ID: MustParse("00000000-0000-0000-0000-000000000001")})
		require.NoError(t, err)
		require.JSONEq(t, `{"ID":"00000000-0000-0000-0000-000000000001"}`, string(b))
	})
}

func TestAppendText(t *testing.T) {
	t.Run("should append to an empty buffer", func(t *testing.T) {
		b, err := MustParse("11223344-5566-7788-99aa-bbccddeeff00").AppendText(nil)
		require.NoError(t, err)
		require.Equal(t, []byte("11223344-5566-7788-99aa-bbccddeeff00"), b)
	})

	t.Run("should preserve the existing prefix", func(t *testing.T) {
		b, err := MustParse("11223344-5566-7788-99aa-bbccddeeff00").AppendText([]byte("id="))
		require.NoError(t, err)
		require.Equal(t, []byte("id=11223344-5566-7788-99aa-bbccddeeff00"), b)
	})
}

func TestUnmarshalText(t *testing.T) {
	t.Run("should parse the canonical form", func(t *testing.T) {
		var u UUID
		require.NoError(t, u.UnmarshalText([]byte("11223344-5566-7788-99aa-bbccddeeff00")))
		require.Equal(t, MustParse("11223344-5566-7788-99aa-bbccddeeff00"), u)
	})

	t.Run("should return an error for malformed input", func(t *testing.T) {
		var u UUID
		require.Error(t, u.UnmarshalText([]byte("not-a-uuid")))
	})

	t.Run("should decode from a json string", func(t *testing.T) {
		var decoded struct{ ID UUID }
		require.NoError(t, json.Unmarshal([]byte(`{"ID":"00000000-0000-0000-0000-000000000001"}`), &decoded))
		require.Equal(t, MustParse("00000000-0000-0000-0000-000000000001"), decoded.ID)
	})

	t.Run("should round trip through json", func(t *testing.T) {
		in := struct{ ID UUID }{ID: NewV7()}
		b, err := json.Marshal(in)
		require.NoError(t, err)

		var out struct{ ID UUID }
		require.NoError(t, json.Unmarshal(b, &out))
		require.Equal(t, in, out)
	})
}
