package jsonx_test

import (
	"math"
	"testing"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
)

func TestRoundTripIdempotent(t *testing.T) {
	type payload struct {
		Name string
		ID   uint64
	}

	t.Run("idempotent", func(t *testing.T) {
		in := payload{Name: "idempotent", ID: math.MaxUint64}

		first, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"Name":"idempotent","ID":18446744073709551615}`, string(first))

		var mid payload
		require.NoError(t, jsonx.Unmarshal(first, &mid))
		require.Equal(t, in, mid)

		second, err := jsonx.Marshal(&mid)
		require.NoError(t, err)
		require.Equal(t, first, second)
	})

	t.Run("when fields are not present during decoding they should not override values in the destination", func(t *testing.T) {
		const empty = "{}"
		in := payload{Name: "idempotent", ID: math.MaxUint64}

		require.NoError(t, jsonx.Unmarshal([]byte(empty), &in))
		require.Equal(t, in.Name, "idempotent")
		require.EqualValues(t, in.ID, uint64(math.MaxUint64))
	})
}

func TestRoundTripArray(t *testing.T) {
	type payload struct {
		Items []string
	}

	t.Run("nil", func(t *testing.T) {
		in := payload{Items: nil}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"Items":[]}`, string(encoded))

		var out payload
		require.NoError(t, jsonx.Unmarshal(encoded, &out))
		require.Equal(t, payload{Items: []string{}}, out)
	})

	t.Run("empty", func(t *testing.T) {
		in := payload{Items: []string{}}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"Items":[]}`, string(encoded))

		var out payload
		require.NoError(t, jsonx.Unmarshal(encoded, &out))
		require.Equal(t, in, out)
	})

	t.Run("items", func(t *testing.T) {
		in := payload{Items: []string{"a", "b", "c"}}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"Items":["a","b","c"]}`, string(encoded))

		var out payload
		require.NoError(t, jsonx.Unmarshal(encoded, &out))
		require.Equal(t, in, out)
	})
}

func TestRoundTripUint64(t *testing.T) {
	type payload struct {
		ID uint64
	}

	t.Run("min", func(t *testing.T) {
		in := payload{ID: 0}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"ID":0}`, string(encoded))

		var out payload
		require.NoError(t, jsonx.Unmarshal(encoded, &out))
		require.Equal(t, in, out)
	})

	t.Run("0", func(t *testing.T) {
		in := payload{ID: 0}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"ID":0}`, string(encoded))

		var out payload
		require.NoError(t, jsonx.Unmarshal(encoded, &out))
		require.Equal(t, in, out)
	})

	t.Run("max", func(t *testing.T) {
		in := payload{ID: math.MaxUint64}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"ID":18446744073709551615}`, string(encoded))

		var out payload
		require.NoError(t, jsonx.Unmarshal(encoded, &out))
		require.Equal(t, in, out)
	})
}
