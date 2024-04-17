package httpx

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func ChaosRateLimited(max time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Retry-After", fmt.Sprintf("%d", max.Truncate(time.Second)))
		w.WriteHeader(http.StatusTooManyRequests)
	})
}

func ChaosStatusCodes(codes ...int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := codes[rand.Intn(len(codes))]
		w.WriteHeader(code)
	})
}

// Chaos inject random errors into the application
// enabled only in dev environments. rate is the percentage
// of request to mess with.
func Chaos(rate float64, behavior ...http.Handler) func(http.Handler) http.Handler {
	if rate == 0 || len(behavior) == 0 {
		return func(original http.Handler) http.Handler {
			return original
		}
	}

	return func(original http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if rate < rand.Float64() {
				original.ServeHTTP(resp, req)
				return
			}

			log.Println("generating chaos event")
			n := behavior[rand.Intn(len(behavior))]
			n.ServeHTTP(resp, req)
		})
	}
}
