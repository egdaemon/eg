package jsonx

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// protoEnumUnmarshal leniently decodes a proto enum from either its declared
// value name (e.g. "LISTED") or its underlying number, matching the proto3 json mapping's
// decoding behavior.
func protoEnumUnmarshal() *json.Unmarshalers {
	return json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v protoreflect.Enum) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}

		var n protoreflect.EnumNumber
		if tok.Kind() == '"' {
			evd := v.Descriptor().Values().ByName(protoreflect.Name(tok.String()))
			if evd == nil {
				return fmt.Errorf("unknown enum value %q for %s", tok.String(), v.Descriptor().FullName())
			}
			n = evd.Number()
		} else {
			i, err := strconv.ParseInt(tok.String(), 10, 32)
			if err != nil {
				return err
			}
			n = protoreflect.EnumNumber(i)
		}

		reflect.ValueOf(v).Elem().SetInt(int64(n))
		return nil
	})
}
