package httpx

import (
	"context"
	"net/http"
	"time"
)

// Timeout10s 10 second timeout handler
func Timeout10s() func(http.Handler) http.Handler {
	return TimeoutHandler(10 * time.Second)
}

// Timeout1s 1 second timeout handler
func Timeout1s() func(http.Handler) http.Handler {
	return TimeoutHandler(time.Second)
}

// Timeout2s 2 second timeout handler
func Timeout2s() func(http.Handler) http.Handler {
	return TimeoutHandler(2 * time.Second)
}

// Timeout4s 4 second timeout handler
func Timeout4s() func(http.Handler) http.Handler {
	return TimeoutHandler(4 * time.Second)
}

// TimeoutHandler inserts a buffer into the http.Request context.
func TimeoutHandler(max time.Duration) func(http.Handler) http.Handler {
	return func(original http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx, cancel := context.WithTimeout(req.Context(), max)
			defer cancel()
			original.ServeHTTP(resp, req.WithContext(ctx))
		})
	}
}
