package daemons

import (
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

// NewCancelHandler constructs the POST /c/cancel/{id} handler, using the default
// on-disk spool directories and the runner's in-memory jwt secret.
func NewCancelHandler() *CancelHandler {
	return &CancelHandler{
		Dirs:   runners.DefaultSpoolDirs(),
		Secret: JWTSecret(),
	}
}

// CancelHandler implements POST /c/cancel/{id}: it requests cancellation of the workload,
// running workloads are stopped, queued workloads are never started. the request must present a
// log token (computeapi.LogClaims) for the workload signed with this runner's secret, the same credential as GET /c/logs/{id}.
// cancelling an already cancelled workload succeeds, an unknown workload is a 404.
// Dirs/Secret are exported so tests can point this at an isolated SpoolDirs and a known secret.
type CancelHandler struct {
	Dirs   runners.SpoolDirs
	Secret jwtx.JWTSecretSource
}

func (t *CancelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if err = t.Dirs.Cancel(uid); errorsx.Is(err, runners.ErrWorkloadNotFound) {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	} else if err != nil {
		log.Println(errorsx.Wrapf(err, "unable to cancel workload: %s", uid))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusAccepted))
}
