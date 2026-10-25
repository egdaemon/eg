package jsonx

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strconv"
)

// lenientInt64 leniently accepts int64 values encoded either as a JSON
// number or as a JSON string containing the number (the convention used by the
// proto3 json mapping to avoid precision loss in clients that decode JSON numbers as
// float64).
func lenientInt64() *json.Unmarshalers {
	return json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v *int64) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}

		if tok.Kind() == '"' {
			n, err := strconv.ParseInt(tok.String(), 10, 64)
			if err != nil {
				return err
			}
			*v = n
			return nil
		}

		n, err := tok.Int()
		if err != nil {
			return err
		}
		*v = n
		return nil
	})
}

// lenientUint64 leniently accepts uint64 values encoded either as a JSON
// number or as a JSON string containing the number (the convention used by the
// proto3 json mapping to avoid precision loss in clients that decode JSON numbers as
// float64).
func lenientUint64() *json.Unmarshalers {
	return json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v *uint64) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}

		if tok.Kind() == '"' {
			n, err := strconv.ParseUint(tok.String(), 10, 64)
			if err != nil {
				return err
			}
			*v = n
			return nil
		}

		n, err := tok.Uint()
		if err != nil {
			return err
		}
		*v = n
		return nil
	})
}
