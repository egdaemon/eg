package jsonx_test

import (
	"testing"

	"github.com/egdaemon/eg/jsonx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestProtoMessage(t *testing.T) {
	t.Run("decodes json_name keys and string encoded 64 bit integers", func(t *testing.T) {
		const doc = `{"name":"derp","number":"9","type":"TYPE_STRING","jsonName":"dp"}`

		var out descriptorpb.FieldDescriptorProto
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, "derp", out.GetName())
		require.EqualValues(t, 9, out.GetNumber())
		require.Equal(t, descriptorpb.FieldDescriptorProto_TYPE_STRING, out.GetType())
		require.Equal(t, "dp", out.GetJsonName())
	})

	// decoding merges into the destination instead of resetting it, keeping
	// any defaults the caller populated.
	t.Run("absent members do not override values in the destination", func(t *testing.T) {
		out := descriptorpb.DescriptorProto{
			Name:    proto.String("kept"),
			Options: &descriptorpb.MessageOptions{Deprecated: proto.Bool(true)},
		}

		require.NoError(t, jsonx.Unmarshal([]byte(`{"reservedName":["a"]}`), &out))
		require.Equal(t, "kept", out.GetName())
		require.NotNil(t, out.Options)
		require.True(t, out.GetOptions().GetDeprecated())
		require.Equal(t, []string{"a"}, out.GetReservedName())
	})

	t.Run("null leaves the destination as is", func(t *testing.T) {
		out := descriptorpb.FieldDescriptorProto{Name: proto.String("kept")}

		require.NoError(t, jsonx.Unmarshal([]byte(`null`), &out))
		require.Equal(t, "kept", out.GetName())
	})

	t.Run("nested within plain structs", func(t *testing.T) {
		type payload struct {
			Field *descriptorpb.FieldDescriptorProto
		}

		out := payload{Field: &descriptorpb.FieldDescriptorProto{Name: proto.String("kept")}}
		require.NoError(t, jsonx.Unmarshal([]byte(`{"Field":{"number":"1"}}`), &out))
		require.Equal(t, "kept", out.Field.GetName())
		require.EqualValues(t, 1, out.Field.GetNumber())
	})

	t.Run("accepts field names, numbers for 64 bit integers, enum numbers and ignores unknown members", func(t *testing.T) {
		var out descriptorpb.UninterpretedOption
		require.NoError(t, jsonx.Unmarshal([]byte(`{"positive_int_value":9,"negativeIntValue":"-3","doubleValue":"1.5","stringValue":"aGk=","unknown":{"a":[1,2]}}`), &out))
		require.EqualValues(t, 9, out.GetPositiveIntValue())
		require.EqualValues(t, -3, out.GetNegativeIntValue())
		require.Equal(t, 1.5, out.GetDoubleValue())
		require.Equal(t, []byte("hi"), out.GetStringValue())

		var field descriptorpb.FieldDescriptorProto
		require.NoError(t, jsonx.Unmarshal([]byte(`{"type":9,"label":"LABEL_REPEATED"}`), &field))
		require.Equal(t, descriptorpb.FieldDescriptorProto_TYPE_STRING, field.GetType())
		require.Equal(t, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, field.GetLabel())
	})

	t.Run("rejects malformed members", func(t *testing.T) {
		var out descriptorpb.FieldDescriptorProto
		require.Error(t, jsonx.Unmarshal([]byte(`{"number":"derp"}`), &out))
	})
}
