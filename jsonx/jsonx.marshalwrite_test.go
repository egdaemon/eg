package jsonx_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
)

func TestMarshalWrite(t *testing.T) {
	type payload struct {
		Name string
		ID   uint64
	}

	t.Run("matches Marshal", func(t *testing.T) {
		in := payload{Name: "match", ID: math.MaxUint64}

		viaMarshal, err := jsonx.Marshal(&in)
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, jsonx.MarshalWrite(&buf, &in))
		require.Equal(t, viaMarshal, buf.Bytes())
	})
}
