package historyapi

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/history"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
)

// NewHTTPHistory constructs the history service over the database, authorizing requests with log tokens
// (runners.LogClaims) signed with the secret, the runner's in-memory jwt secret.
func NewHTTPHistory(db *sql.DB, secret jwtx.JWTSecretSource) *HistoryService {
	return &HistoryService{
		db:     db,
		secret: secret,
	}
}

// HistoryService implements GET /{id} and GET /{id}/logs: the recorded outcome
// and the logs of a workload this runner completed, until the record expires (history.DefaultTTL).
// the request must present a log token (runners.LogClaims) for the workload signed with this runner's secret.
type HistoryService struct {
	db     *sql.DB
	secret jwtx.JWTSecretSource
}

// Bind the routes, the router is expected to be rooted at /c/history.
func (t *HistoryService) Bind(r *mux.Router) {
	r.StrictSlash(false)
	r.Use(httpx.RouteInvoked)

	r.Path("/{id}/logs").Methods(http.MethodGet).HandlerFunc(t.logs)
	r.Path("/{id}").Methods(http.MethodGet).HandlerFunc(t.find)
}

// find serves the record of the workload.
func (t *HistoryService) find(w http.ResponseWriter, r *http.Request) {
	uid, ok := t.authorize(w, r)
	if !ok {
		return
	}

	var rec history.Entry
	err := history.EntryFindByID(r.Context(), t.db, uid.String()).Scan(&rec)
	if errors.Is(err, sql.ErrNoRows) {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	} else if err != nil {
		log.Println(errorsx.Wrap(err, "unable to find history"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	errorsx.Log(httpx.WriteJSON(w, &bytes.Buffer{}, &compute.HistoryFindResponse{
		History: &compute.History{
			Id:          rec.ID,
			Description: rec.Description,
			CreatedAt:   rec.CreatedAt.UTC().Format(time.RFC3339),
			InitiatedAt: rec.InitiatedAt.UTC().Format(time.RFC3339),
			CompletedAt: rec.CompletedAt.UTC().Format(time.RFC3339),
			Duration:    rec.Duration,
			Result:      rec.Result,
			UploadedBy:  rec.UploadedBy,
		},
	}))
}

// logs serves the recorded logs of the workload.
func (t *HistoryService) logs(w http.ResponseWriter, r *http.Request) {
	uid, ok := t.authorize(w, r)
	if !ok {
		return
	}

	var rec history.Entry
	err := history.EntryFindByID(r.Context(), t.db, uid.String()).Scan(&rec)
	if errors.Is(err, sql.ErrNoRows) {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	} else if err != nil {
		log.Println(errorsx.Wrap(err, "unable to find history"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if rec.LogsPath == "" {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	}

	f, err := os.Open(rec.LogsPath)
	if errors.Is(err, os.ErrNotExist) {
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusNotFound))
		return
	} else if err != nil {
		log.Println(errorsx.Wrap(err, "unable to open history logs"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	// headers are sent; on failure the stream just ends.
	_, err = io.Copy(w, f)
	errorsx.Log(errorsx.Wrap(err, "unable to stream history logs"))
}

// authorize validates the log token against the workload in the path, writing the failure response when it isn't valid.
func (t *HistoryService) authorize(w http.ResponseWriter, r *http.Request) (uid uuid.UUID, ok bool) {
	var (
		err    error
		claims runners.LogClaims
	)

	if uid, err = uuid.FromString(mux.Vars(r)["id"]); err != nil {
		log.Println(errorsx.Wrap(err, "invalid workload id"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return uid, false
	}

	if err = jwtx.Validate(t.secret, r.Header.Get("Authorization"), &claims); err != nil {
		log.Println(errorsx.Wrap(err, "invalid log token"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusUnauthorized))
		return uid, false
	}

	if uuid.FromStringOrNil(claims.Workload) != uid {
		log.Println("log token not valid for workload", uid, claims.Workload)
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusForbidden))
		return uid, false
	}

	return uid, true
}
