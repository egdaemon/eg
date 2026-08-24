package gdx

import (
	"context"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/egdaemon/gdx/internal/errorsx"
	"github.com/egdaemon/gdx/internal/langx"
	"github.com/egdaemon/gdx/konggdx/userx"
	"github.com/gorilla/mux"
)

const DefaultSocket = "gdx.socket"

// autosocket builds the default unix socket path:
// ${RUNTIME_DIR}/${binname}/gdx.socket.
// binname must be a basename, not a full path (use filepath.Base).
func autosocket(binname string) string {
	return userx.RuntimeDirectory(filepath.Base(binname), DefaultSocket)
}

// AutoSocket returns the default unix socket path derived from os.Args[0].
func AutoSocket() string {
	return autosocket(langx.FirstNonZero(os.Args...))
}

func AutoUnixServe(ctx context.Context, options ...option) {
	UnixServe(ctx, AutoSocket(), options...)
}

func UnixServe(ctx context.Context, gdxpath string, options ...option) {
	errorsx.Log(errorsx.Wrap(errorsx.Ignore(os.Remove(gdxpath), os.ErrNotExist), "failed to remove previous gdx.socket"))
	// best effort for ensuring directory exists. if this fails the socket will fail too.
	errorsx.Log(errorsx.Wrap(os.MkdirAll(filepath.Dir(gdxpath), 0700), "failed to create socket directory"))

	l, err := net.Listen("unix", gdxpath)
	if err != nil {
		log.Println("unable to bind gdx debug socket", err)
		return
	}

	ctx, done := context.WithCancel(ctx)
	defer done()
	go func() {
		<-ctx.Done()
		errorsx.Log(errorsx.Wrap(l.Close(), "gdx shutdown"))
	}()

	log.Println("gdx debug available at:", gdxpath)
	if err := http.Serve(l, NewHTTPFn(options...)); langx.FirstNonNil(err, ctx.Err()) == nil {
		log.Println("gdx debug server stopped", langx.FirstNonNil(err, ctx.Err()))
		return
	}
}

// NewHTTPFn builds a stdlib http.Handler exposing the diagx debug surface
// (goroutine dumps, profiles, traces, expvar). diagx never binds a listener
// itself: mount the returned handler however the caller likes, e.g.
//
//	http.Serve(net.Listen("unix", path), diagx.NewHTTPFn(diagx.Options().FromEnv()))
func NewHTTPFn(opts ...option) http.Handler {
	cfg := options(opts).apply()

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
