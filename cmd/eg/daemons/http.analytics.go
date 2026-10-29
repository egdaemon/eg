package daemons

import (
	"errors"
	"io/fs"
	"log"
	"net/http"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
)

// NewAnalyticsHandler constructs the GET /c/analytics/{id} handler, using the default
// on-disk spool directories and the runner's in-memory jwt secret.
func NewAnalyticsHandler() *AnalyticsHandler {
	return &AnalyticsHandler{
		Dirs:   runners.DefaultSpoolDirs(),
		Secret: JWTSecret(),
	}
}

// AnalyticsHandler implements GET /c/analytics/{id}: it serves a snapshot of the analytics
// database of the running workload. the request must present a log token (computeapi.LogClaims)
// for the workload signed with this runner's secret. completed workloads are served by the
// history routes, this responds 404 for them.
// Dirs/Secret are exported so tests can point this at an isolated SpoolDirs
// and a known secret instead of the process defaults.
type AnalyticsHandler struct {
	Dirs   runners.SpoolDirs
	Secret jwtx.JWTSecretSource
}

func (t *AnalyticsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		uid    uuid.UUID
		claims computeapi.LogClaims
	)

	if uid, err = uuid.FromString(mux.Vars(r)["id"]); err != nil {
		log.Println(errorsx.Wrap(err, "invalid workload id"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	if err = jwtx.Validate(t.Secret, r.Header.Get("Authorization"), &claims); err != nil {
		log.Println(errorsx.Wrap(err, "invalid log token"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusUnauthorized))
		return
	}

	if uuid.FromStringOrNil(claims.Workload) != uid {
		log.Println("log token not valid for workload", uid, claims.Workload)
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusForbidden))
		return
	}

	if _, ok := t.Dirs.Locate(uid); !ok {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")

	// the snapshot is staged before any bytes are sent so failures can still report a status.
	staged := &statusWriter{ResponseWriter: w}
	if err = t.Dirs.SnapshotAnalytics(r.Context(), uid, staged); errors.Is(err, fs.ErrNotExist) && !staged.started {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	} else if err != nil && !staged.started {
		log.Println(errorsx.Wrapf(err, "analytics snapshot failed: %s", uid))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	} else if err != nil {
		log.Println(errorsx.Wrapf(err, "analytics stream failed: %s", uid))
	}
}

// statusWriter notes when the response body started so failures before then can still set a status.
type statusWriter struct {
	http.ResponseWriter
	started bool
}

func (t *statusWriter) Write(b []byte) (int, error) {
	t.started = true
	return t.ResponseWriter.Write(b)
}
