package jsonx_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	type payload struct {
		Name string
		ID   uint64
	}

	t.Run("ignores unknown fields", func(t *testing.T) {
		const doc = `{"unknown":"derp","Name":"kept","ID":42}`

		var out payload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, payload{Name: "kept", ID: 42}, out)
	})

	t.Run("matches field names case insensitively", func(t *testing.T) {
		const doc = `{"name":"kept","id":42}`

		var out payload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, payload{Name: "kept", ID: 42}, out)
	})

	t.Run("prefers the exactly matching field over a case insensitive one", func(t *testing.T) {
		type tagged struct {
			Lower string `json:"name"`
			Upper string `json:"Name"`
		}

		const doc = `{"name":"lower","Name":"upper"}`

		var out tagged
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, tagged{Lower: "lower", Upper: "upper"}, out)
	})

	t.Run("accepts string-encoded uint64", func(t *testing.T) {
		type uintPayload struct {
			ID uint64
		}

		const doc = `{"ID":"18446744073709551615"}`

		var out uintPayload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, uintPayload{ID: 18446744073709551615}, out)
	})

	t.Run("accepts number-encoded uint64", func(t *testing.T) {
		type uintPayload struct {
			ID uint64
		}

		const doc = `{"ID":18446744073709551615}`

		var out uintPayload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, uintPayload{ID: 18446744073709551615}, out)
	})

	t.Run("accepts string-encoded int64", func(t *testing.T) {
		type intPayload struct {
			ID int64
		}

		const doc = `{"ID":"-9223372036854775808"}`

		var out intPayload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, intPayload{ID: -9223372036854775808}, out)
	})

	t.Run("accepts number-encoded int64", func(t *testing.T) {
		type intPayload struct {
			ID int64
		}

		const doc = `{"ID":-9223372036854775808}`

		var out intPayload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, intPayload{ID: -9223372036854775808}, out)
	})

	t.Run("accepts nanosecond-encoded time.Duration", func(t *testing.T) {
		type record struct {
			TTL time.Duration
		}

		const doc = `{"TTL":60000000000}`

		var out record
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, record{TTL: time.Minute}, out)
	})
}
