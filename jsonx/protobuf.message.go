package jsonx

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"math"
	"strconv"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// protoMessageUnmarshal decodes protobuf messages using the proto3 json mapping (json name or field
// name keys, 64 bit integers as strings or numbers, enum names or numbers, base64 bytes), ignoring
// unknown members. like encoding/json it decodes into the destination instead of resetting it:
// members absent from the payload leave existing values untouched, lists and maps present replace
// the destination's, null resets the field.
//
// registered unmarshalers take precedence over a type's UnmarshalJSON method.
func protoMessageUnmarshal() *json.Unmarshalers {
	return json.UnmarshalFromFunc(func(dec *jsontext.Decoder, m proto.Message) error {
		// null leaves the destination as is.
		if dec.PeekKind() == 'n' {
			_, err := dec.ReadToken()
			return err
		}

		return readMessage(dec, m.ProtoReflect())
	})
}

// specialMapping is true for the well known types the proto3 json mapping encodes as something
// other than an object of their fields; none are used by egmeta.
func specialMapping(d protoreflect.MessageDescriptor) bool {
	switch d.FullName() {
	case "google.protobuf.Any", "google.protobuf.Timestamp", "google.protobuf.Duration", "google.protobuf.Struct",
		"google.protobuf.Value", "google.protobuf.ListValue", "google.protobuf.FieldMask",
		"google.protobuf.BoolValue", "google.protobuf.BytesValue", "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
		"google.protobuf.Int32Value", "google.protobuf.Int64Value", "google.protobuf.StringValue",
		"google.protobuf.UInt32Value", "google.protobuf.UInt64Value":
		return true
	}
	return false
}

func readMessage(dec *jsontext.Decoder, m protoreflect.Message) error {
	if specialMapping(m.Descriptor()) {
		return fmt.Errorf("well known type %s is not supported", m.Descriptor().FullName())
	}

	if tok, err := dec.ReadToken(); err != nil {
		return err
	} else if tok.Kind() != '{' {
		return fmt.Errorf("expected an object for %s, got %v", m.Descriptor().FullName(), tok.Kind())
	}

	fields := m.Descriptor().Fields()
	for dec.PeekKind() != '}' {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}

		key := tok.String()
		fd := fields.ByJSONName(key)
		if fd == nil {
			fd = fields.ByName(protoreflect.Name(key))
		}

		// unknown members are discarded.
		if fd == nil {
			if err = dec.SkipValue(); err != nil {
				return err
			}
			continue
		}

		// null resets the field to its default value.
		if dec.PeekKind() == 'n' {
			if _, err = dec.ReadToken(); err != nil {
				return err
			}
			m.Clear(fd)
			continue
		}

		switch {
		case fd.IsList():
			m.Clear(fd)
			err = readList(dec, fd, m.Mutable(fd).List())
		case fd.IsMap():
			m.Clear(fd)
			err = readMap(dec, fd, m.Mutable(fd).Map())
		case fd.Message() != nil:
			err = readMessage(dec, m.Mutable(fd).Message())
		default:
			var v protoreflect.Value
			if v, err = readScalar(dec, fd); err == nil {
				m.Set(fd, v)
			}
		}
		if err != nil {
			return fmt.Errorf("%s: %w", fd.FullName(), err)
		}
	}

	_, err := dec.ReadToken()
	return err
}

func readList(dec *jsontext.Decoder, fd protoreflect.FieldDescriptor, l protoreflect.List) error {
	if tok, err := dec.ReadToken(); err != nil {
		return err
	} else if tok.Kind() != '[' {
		return fmt.Errorf("expected an array, got %v", tok.Kind())
	}

	for dec.PeekKind() != ']' {
		if fd.Message() != nil {
			elem := l.NewElement()
			if err := readMessage(dec, elem.Message()); err != nil {
				return err
			}
			l.Append(elem)
			continue
		}

		v, err := readScalar(dec, fd)
		if err != nil {
			return err
		}
		l.Append(v)
	}

	_, err := dec.ReadToken()
	return err
}

func readMap(dec *jsontext.Decoder, fd protoreflect.FieldDescriptor, m protoreflect.Map) error {
	if tok, err := dec.ReadToken(); err != nil {
		return err
	} else if tok.Kind() != '{' {
		return fmt.Errorf("expected an object, got %v", tok.Kind())
	}

	for dec.PeekKind() != '}' {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}

		k, err := parseMapKey(fd.MapKey(), tok.String())
		if err != nil {
			return err
		}

		if fd.MapValue().Message() != nil {
			elem := m.NewValue()
			if err = readMessage(dec, elem.Message()); err != nil {
				return err
			}
			m.Set(k, elem)
			continue
		}

		v, err := readScalar(dec, fd.MapValue())
		if err != nil {
			return err
		}
		m.Set(k, v)
	}

	_, err := dec.ReadToken()
	return err
}

func parseMapKey(fd protoreflect.FieldDescriptor, s string) (protoreflect.MapKey, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(s).MapKey(), nil
	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(s)
		return protoreflect.ValueOfBool(b).MapKey(), err
	}

	v, err := parseInteger(fd.Kind(), s)
	return v.MapKey(), err
}

func readScalar(dec *jsontext.Decoder, fd protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return protoreflect.Value{}, err
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		if tok.Kind() != 't' && tok.Kind() != 'f' {
			return protoreflect.Value{}, fmt.Errorf("expected a boolean, got %v", tok.Kind())
		}
		return protoreflect.ValueOfBool(tok.Bool()), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if tok.Kind() != '"' && tok.Kind() != '0' {
			return protoreflect.Value{}, fmt.Errorf("expected an integer, got %v", tok.Kind())
		}
		return parseInteger(fd.Kind(), tok.String())
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		if tok.Kind() != '"' && tok.Kind() != '0' {
			return protoreflect.Value{}, fmt.Errorf("expected a number, got %v", tok.Kind())
		}
		return parseFloat(fd.Kind(), tok.String())
	case protoreflect.StringKind:
		if tok.Kind() != '"' {
			return protoreflect.Value{}, fmt.Errorf("expected a string, got %v", tok.Kind())
		}
		return protoreflect.ValueOfString(tok.String()), nil
	case protoreflect.BytesKind:
		if tok.Kind() != '"' {
			return protoreflect.Value{}, fmt.Errorf("expected a base64 string, got %v", tok.Kind())
		}
		return parseBytes(tok.String())
	case protoreflect.EnumKind:
		switch tok.Kind() {
		case '"':
			ev := fd.Enum().Values().ByName(protoreflect.Name(tok.String()))
			if ev == nil {
				return protoreflect.Value{}, fmt.Errorf("unknown enum value %q for %s", tok.String(), fd.Enum().FullName())
			}
			return protoreflect.ValueOfEnum(ev.Number()), nil
		case '0':
			n, err := strconv.ParseInt(tok.String(), 10, 32)
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(n)), err
		}
		return protoreflect.Value{}, fmt.Errorf("expected an enum name or number, got %v", tok.Kind())
	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind %s", fd.Kind())
	}
}

// parseInteger accepts the text of a json number or string, also integral values written with an
// exponent or fraction (1e3, 5.0).
func parseInteger(kind protoreflect.Kind, s string) (protoreflect.Value, error) {
	var (
		bits   = 64
		signed = true
	)

	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		bits = 32
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		bits, signed = 32, false
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		signed = false
	}

	if signed {
		n, err := strconv.ParseInt(s, 10, bits)
		if err != nil {
			f, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil || f != math.Trunc(f) || f < -math.Pow(2, float64(bits-1)) || f >= math.Pow(2, float64(bits-1)) {
				return protoreflect.Value{}, fmt.Errorf("invalid integer %q", s)
			}
			n = int64(f)
		}

		if bits == 32 {
			return protoreflect.ValueOfInt32(int32(n)), nil
		}
		return protoreflect.ValueOfInt64(n), nil
	}

	n, err := strconv.ParseUint(s, 10, bits)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil || f != math.Trunc(f) || f < 0 || f >= math.Pow(2, float64(bits)) {
			return protoreflect.Value{}, fmt.Errorf("invalid integer %q", s)
		}
		n = uint64(f)
	}

	if bits == 32 {
		return protoreflect.ValueOfUint32(uint32(n)), nil
	}
	return protoreflect.ValueOfUint64(n), nil
}

func parseFloat(kind protoreflect.Kind, s string) (protoreflect.Value, error) {
	var f float64
	switch s {
	case "NaN":
		f = math.NaN()
	case "Infinity":
		f = math.Inf(1)
	case "-Infinity":
		f = math.Inf(-1)
	default:
		var err error
		bits := 64
		if kind == protoreflect.FloatKind {
			bits = 32
		}

		if f, err = strconv.ParseFloat(s, bits); err != nil {
			return protoreflect.Value{}, fmt.Errorf("invalid number %q", s)
		}
	}

	if kind == protoreflect.FloatKind {
		return protoreflect.ValueOfFloat32(float32(f)), nil
	}
	return protoreflect.ValueOfFloat64(f), nil
}

func parseBytes(s string) (protoreflect.Value, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return protoreflect.ValueOfBytes(b), nil
		}
	}

	return protoreflect.Value{}, fmt.Errorf("invalid base64 value")
}
