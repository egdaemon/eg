package daemons

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
)

// NewLogsHandler constructs the GET /c/logs/{id} handler, using the default
// on-disk spool directories and the runner's in-memory jwt secret.
func NewLogsHandler() *LogsHandler {
	return &LogsHandler{
		Dirs:   runners.DefaultSpoolDirs(),
		Secret: JWTSecret(),
	}
}

// LogsHandler implements GET /c/logs/{id}: it streams the log of the workload
// as it is written (tail -f) until the workload completes or the client goes
// away. the request must present a log token (computeapi.LogClaims) for the
// workload signed with this runner's secret. the optional offset query
// parameter resumes the stream from the given byte offset.
// Dirs/Secret are exported so tests can point this at an isolated SpoolDirs
// and a known secret instead of the process defaults.
type LogsHandler struct {
	Dirs   runners.SpoolDirs
	Secret jwtx.JWTSecretSource
}

func (t *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		uid    uuid.UUID
		offset int64
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

	if s := r.URL.Query().Get("offset"); !stringsx.Blank(s) {
		if offset, err = strconv.ParseInt(s, 10, 64); err != nil || offset < 0 {
			log.Println("invalid log offset", s)
			errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}
	}

	if _, ok := t.Dirs.Locate(uid); !ok {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	flush := func() {
		errorsx.Log(errorsx.Wrap(rc.Flush(), "unable to flush logs"))
	}
	flush()

	// headers are sent; on failure the stream just ends.
	if err = t.Dirs.Follow(r.Context(), uid, offset, w, flush); err != nil && !errors.Is(err, context.Canceled) {
		log.Println(errorsx.Wrapf(err, "log stream failed: %s", uid))
	}
}
