package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// EncodeJSON encode data into the http.Request body.
func EncodeJSON(req *http.Request, body interface{}) (err error) {
	var (
		encoded []byte
	)

	if encoded, err = json.Marshal(body); err != nil {
		return errors.WithStack(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(encoded))

	return nil
}
