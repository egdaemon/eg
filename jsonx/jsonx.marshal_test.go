package jsonx_test

import (
	"math"
	"testing"
	"time"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	type payload struct {
		Name string
		ID   uint64
	}

	t.Run("uint64 encodes as exact number", func(t *testing.T) {
		in := payload{Name: "derp", ID: math.MaxUint64}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)
		require.Equal(t, `{"Name":"derp","ID":18446744073709551615}`, string(encoded))
	})

	// v2 narrowed omitempty to mean "encodes to an empty JSON value", so a zero
	// number or false no longer drops out. protobuf generated structs tag every
	// field omitempty, so without v1's meaning every payload gains its zero
	// valued fields back.
	t.Run("omitempty drops zero values the way encoding/json does", func(t *testing.T) {
		type record struct {
			Name  string `json:"name,omitempty"`
			Bytes uint64 `json:"bytes,omitempty"`
			Adult bool   `json:"adult,omitempty"`
		}

		encoded, err := jsonx.Marshal(&record{Name: "derp"})
		require.NoError(t, err)
		require.Equal(t, `{"name":"derp"}`, string(encoded))
	})

	t.Run("struct with no exported fields encodes as empty object", func(t *testing.T) {
		type empty struct {
			unexported string
		}

		encoded, err := jsonx.Marshal(&empty{unexported: "derp"})
		require.NoError(t, err)
		require.Equal(t, `{}`, string(encoded))
	})

	// v2 has no default representation for time.Duration and errors instead
	// of guessing, so this package configures it to encode as its raw
	// nanosecond count, matching v1's behavior.
	t.Run("time.Duration encodes as nanoseconds", func(t *testing.T) {
		type record struct {
			TTL time.Duration
		}

		encoded, err := jsonx.Marshal(&record{TTL: time.Minute})
		require.NoError(t, err)
		require.Equal(t, `{"TTL":60000000000}`, string(encoded))
	})
}
