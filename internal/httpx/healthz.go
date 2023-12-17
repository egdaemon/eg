package httpx

import (
	"log"
	"net/http"
)

// Healthz
func Healthz(code int) http.HandlerFunc {
	log.Println("healthz enabled with", code)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}
}
