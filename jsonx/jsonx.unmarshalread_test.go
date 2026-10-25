package jsonx_test

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalRead(t *testing.T) {
	type payload struct {
		Name string
		ID   uint64
	}

	// encoding/json's Decoder reported a stream with nothing in it as io.EOF,
	// and callers replacing a decoder with this branch on that to treat an
	// absent body as absent rather than malformed.
	t.Run("reports an empty stream as io.EOF", func(t *testing.T) {
		var out payload
		require.ErrorIs(t, jsonx.UnmarshalRead(strings.NewReader(""), &out), io.EOF)
	})

	t.Run("matches Unmarshal", func(t *testing.T) {
		in := payload{Name: "match", ID: math.MaxUint64}

		encoded, err := jsonx.Marshal(&in)
		require.NoError(t, err)

		var a, b payload
		require.NoError(t, jsonx.Unmarshal(encoded, &a))
		require.NoError(t, jsonx.UnmarshalRead(bytes.NewReader(encoded), &b))
		require.Equal(t, a, b)
	})
}
