package gdx

import (
	"context"
	"expvar"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

const DefaultSocket = "gdx.socket"

// NewHTTPFn builds a stdlib http.Handler exposing the diagx debug surface
// (goroutine dumps, profiles, traces, expvar). diagx never binds a listener
// itself: mount the returned handler however the caller likes, e.g.
//
//	http.Serve(net.Listen("unix", path), diagx.NewHTTPFn(diagx.Options().FromEnv()))
func NewHTTPFn(opts options) http.Handler {
	cfg := opts.apply()

	r := mux.NewRouter()
	r.Handle("/debug/vars", expvar.Handler()).Methods(http.MethodGet)
	r.HandleFunc("/debug/goroutines", goroutinesHandler).Methods(http.MethodGet)
	r.HandleFunc("/debug/profile/{mode}", profileHandler(cfg)).Methods(http.MethodGet)
	r.HandleFunc("/debug/trace", traceHandler(cfg)).Methods(http.MethodGet)

	return r
}

func goroutinesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := DumpRoutinesInto(nopCloser{w}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func profileHandler(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := mux.Vars(r)["mode"]
		mode := ProfileModeFromString(raw)

		ctx, cancel := context.WithTimeout(r.Context(), parseDuration(r, cfg.defaultDuration))
		defer cancel()

		w.Header().Set("Content-Type", "application/octet-stream")

		// ProfileModeFromString returns the zero value (ProfileMode_cpu) for an
		// unrecognized name, so round-trip it through String() to distinguish
		// "cpu" from garbage before handing it to Profile.
		if mode.String() != raw {
			http.Error(w, fmt.Sprintf("unknown profile mode: %s", raw), http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(w, Profile(ctx, mode)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func traceHandler(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), parseDuration(r, cfg.defaultDuration))
		defer cancel()

		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := io.Copy(w, Trace(ctx)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func parseDuration(r *http.Request, fallback time.Duration) time.Duration {
	raw := r.URL.Query().Get("duration")
	if raw == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}
