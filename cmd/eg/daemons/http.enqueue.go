package daemons

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"

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

// EnqueueHandler implements POST /c/enqueue: it accepts a source-ref
// submission (instead of a pre-built archive) pushed to this runner,
// deciding synchronously whether to admit it based on current load before
// spooling it into the compile spool for asynchronous clone+compile (see
// runners.CompileRequest and runners.CompileN). Dirs/RM are exported so
// tests can point this at an isolated SpoolDirs and ResourceManager instead
// of the process defaults.
type EnqueueHandler struct {
	Dirs runners.SpoolDirs
	RM   *runners.ResourceManager
}

func (t *EnqueueHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err error
		uid uuid.UUID
		req runners.CompileRequest
	)

	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println(errorsx.Wrap(err, "unable to decode enqueue request"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	want := runners.RuntimeResources{Cores: req.Cores, Memory: req.Memory, Vram: req.Vram}
	if !t.RM.Admit(want) {
		log.Println("rejecting enqueue request, insufficient capacity", req.VcsUri)
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusConflict))
		return
	}

	if uid, err = uuid.NewV7(); err != nil {
		log.Println(errorsx.Wrap(err, "unable to generate uuid"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	encoded, err := json.Marshal(&req)
	if err != nil {
		log.Println(errorsx.Wrap(err, "unable to encode compile request"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Download(uid, "metadata.json", bytes.NewReader(encoded)); err != nil {
		log.Println(errorsx.Wrap(err, "unable to persist enqueue request"))
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
	errorsx.Log(errorsx.Wrap(json.NewEncoder(w).Encode(map[string]string{"id": uid.String()}), "unable to write response"))
}
