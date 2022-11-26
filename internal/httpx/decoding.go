package httpx

import (
	"encoding/json"
	"net/http"
)

// DecodeJSON from a http.Response into the provide destination.
func DecodeJSON(resp *http.Response, dst interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dst)
}
