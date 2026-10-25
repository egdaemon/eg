package jsonx_test

import (
	"testing"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEnum(t *testing.T) {
	type payload struct {
		Mode descriptorpb.FieldDescriptorProto_Type
	}

	t.Run("marshals as the underlying number", func(t *testing.T) {
		encoded, err := jsonx.Marshal(&payload{Mode: descriptorpb.FieldDescriptorProto_TYPE_STRING})
		require.NoError(t, err)
		require.Equal(t, `{"Mode":9}`, string(encoded))
	})

	t.Run("unmarshals from the declared value name", func(t *testing.T) {
		const doc = `{"Mode":"TYPE_BOOL"}`

		var out payload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, descriptorpb.FieldDescriptorProto_TYPE_BOOL, out.Mode)
	})

	t.Run("unmarshals from the underlying number", func(t *testing.T) {
		const doc = `{"Mode":9}`

		var out payload
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, descriptorpb.FieldDescriptorProto_TYPE_STRING, out.Mode)
	})
}
