package httpx

import "net/http"

// GatedResponse short-circuits the handler chain with the given status code
// when enabled is false, otherwise it passes through to the next handler.
func GatedResponse(enabled bool, code int) func(http.Handler) http.Handler {
	return func(original http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !enabled {
				resp.WriteHeader(code)
				return
			}
			original.ServeHTTP(resp, req)
		})
	}
}
