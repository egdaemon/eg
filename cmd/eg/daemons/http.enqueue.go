package daemons

import (
	"bytes"
	"encoding/json"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
)

// NewEnqueueHandler constructs the POST /c/enqueue handler. dirs is the
// compile spool (distinct from the run spool -- see runners.CompileN), which
// this handler writes pending source-ref submissions into.
func NewEnqueueHandler(dirs runners.SpoolDirs, rm *runners.ResourceManager) *EnqueueHandler {
	return &EnqueueHandler{
		Dirs: dirs,
		RM:   rm,
	}
}

// EnqueueHandler implements POST /c/enqueue: it accepts a multipart
// source-ref submission (instead of a pre-built archive) pushed to this
// runner -- an "enqueued" field (JSON-encoded runners.EnqueuedDequeueResponse,
// the same shape a runner gets from the polling /c/q/dequeue flow: its
// Enqueued.VcsCommit is the treeish to check out, its AccessToken is the
// short-lived token the runner exchanges for actual git credentials via
// gitx.RefreshCredentials -- it does not clone with AccessToken directly)
// plus a required "environ" file (see eg.EnvironFile; may be empty -- only
// carries a fallback ref/token for callers that predate these structured
// fields) -- deciding synchronously whether to admit it based on current
// load before spooling it into the compile spool for asynchronous
// clone+compile (see runners.CompileN). Dirs/RM are exported so tests can
// point this at an isolated SpoolDirs and ResourceManager instead of the
// process defaults.
type EnqueueHandler struct {
	Dirs runners.SpoolDirs
	RM   *runners.ResourceManager
}

func (t *EnqueueHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		uid  uuid.UUID
		req  runners.EnqueuedDequeueResponse
		envc multipart.File
	)

	encoded := []byte(r.FormValue("enqueued"))
	if err = json.Unmarshal(encoded, &req); err != nil {
		log.Println(errorsx.Wrap(err, "unable to decode enqueue request"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	if req.Enqueued == nil {
		log.Println("enqueue request missing enqueued payload")
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	// egmeta already assigned this job a real id (same as the polling
	// /c/q/dequeue flow -- see client.download.go's WorkloadClient.Download,
	// which likewise parses workload.Enqueued.Id rather than minting one),
	// so reuse it as the spool directory name instead of generating a new
	// one.
	if uid, err = uuid.FromString(req.Enqueued.Id); err != nil {
		log.Println(errorsx.Wrap(err, "invalid enqueue request, malformed id"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	want := runners.RuntimeResources{Cores: req.Enqueued.Cores, Memory: req.Enqueued.Memory, Vram: req.Enqueued.Vram}
	if !t.RM.Admit(want) {
		log.Println("rejecting enqueue request, insufficient capacity", req.Enqueued.VcsUri)
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusConflict))
		return
	}

	if envc, _, err = r.FormFile("environ"); err != nil {
		log.Println(errorsx.Wrap(err, "environ file parameter required"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}
	defer envc.Close()

	if err = t.Dirs.Download(uid, "metadata.json", bytes.NewReader(encoded)); err != nil {
		log.Println(errorsx.Wrap(err, "unable to persist enqueue request"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Download(uid, eg.EnvironFile, envc); err != nil {
		log.Println(errorsx.Wrap(err, "unable to persist enqueue request environment"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Enqueue(uid); err != nil {
		log.Println(errorsx.Wrap(err, "unable to enqueue for compile"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	log.Println("enqueued for compile", filepath.Join(t.Dirs.Queued, uid.String()))

	w.WriteHeader(http.StatusAccepted)
	errorsx.Log(errorsx.Wrap(httpx.WriteJSON(w, httpx.GetBuffer(r), req.Enqueued), "unable to write response"))
}
