package daemons

import (
	"bytes"
	"github.com/egdaemon/eg/jsonx"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/flock"
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
// runner -- an "enqueued" field (JSON-encoded computeapi.EnqueuedDequeueResponse,
// the same shape a runner gets from the polling /c/q/dequeue flow: its
// Enqueued.VcsCommit is the treeish to check out, its AccessToken is the
// short-lived token the runner exchanges for actual git credentials via
// gitx.RefreshCredentials -- it does not clone with AccessToken directly)
// plus a required "environ" file (see eg.EnvironFile; may be empty -- only
// carries a fallback ref/token for callers that predate these structured
// fields) and the "token" field, the candidate token the control plane
// issued for this runner, used to reserve the workload once it is compiled and
// to report its progress -- deciding synchronously whether to admit it based on current
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
		req  computeapi.EnqueuedDequeueResponse
		envc multipart.File
	)

	encoded := []byte(r.FormValue("enqueued"))
	if err = jsonx.Unmarshal(encoded, &req); err != nil {
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

	// serialize pushes of the same id. the lock lives outside the spool
	// directories so the download -> queued rename never moves it, and the kernel
	// releases it if we crash, so a free lock with a leftover download directory
	// means a dead upload.
	lockdir := filepath.Join(filepath.Dir(t.Dirs.Downloading), "l")
	if err = os.MkdirAll(lockdir, 0700); err != nil {
		log.Println(errorsx.Wrap(err, "unable to create enqueue lock directory"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	lock := flock.New(filepath.Join(lockdir, runners.Queued().Dirname(uid)+".lock"))
	if locked, lerr := lock.TryLock(); lerr != nil {
		log.Println(errorsx.Wrap(lerr, "unable to lock enqueue request"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	} else if !locked {
		// another request for this id is in flight and will admit it.
		log.Println("enqueue already in progress", uid)
		w.WriteHeader(http.StatusAccepted)
		errorsx.Log(errorsx.Wrap(httpx.WriteJSON(w, httpx.GetBuffer(r), req.Enqueued), "unable to write response"))
		return
	}
	defer func() { errorsx.Log(lock.Unlock()) }()

	if dir, ok := t.Dirs.Locate(uid); ok {
		if dir != filepath.Join(t.Dirs.Downloading, runners.Queued().Dirname(uid)) {
			log.Println("already enqueued", uid)
			w.WriteHeader(http.StatusAccepted)
			errorsx.Log(errorsx.Wrap(httpx.WriteJSON(w, httpx.GetBuffer(r), req.Enqueued), "unable to write response"))
			return
		}

		// we hold the lock, so the download is a dead upload. start fresh.
		if err = t.Dirs.Discard(dir); err != nil {
			log.Println(errorsx.Wrap(err, "unable to discard stale download"))
			errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
			return
		}
	}

	want := runners.RuntimeResources{Cores: req.Enqueued.Cores, Memory: req.Enqueued.Memory, Vram: req.Enqueued.Vram}
	if !t.RM.Admit(want) {
		log.Printf("rejecting enqueue request, insufficient capacity %s current=%+v requested=%+v limit=%+v\n", req.Enqueued.VcsUri, t.RM.Snapshot(), want, t.RM.Limit)
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusConflict))
		return
	}

	candidate := r.FormValue("token")
	if stringsx.Blank(candidate) {
		log.Println("enqueue request missing candidate token")
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
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

	if err = t.Dirs.Download(uid, runners.CandidateTokenFile, strings.NewReader(candidate)); err != nil {
		log.Println(errorsx.Wrap(err, "unable to persist candidate token"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Enqueue(uid); os.IsExist(err) {
		// lost a race with an enqueue we could not see; the job is already queued.
		errorsx.Log(errorsx.Wrap(t.Dirs.Discard(filepath.Join(t.Dirs.Downloading, runners.Queued().Dirname(uid))), "unable to discard duplicate download"))
	} else if err != nil {
		log.Println(errorsx.Wrap(err, "unable to enqueue for compile"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	defer t.Dirs.Wake()

	log.Println("enqueued for compile", filepath.Join(t.Dirs.Queued, uid.String()))

	w.WriteHeader(http.StatusAccepted)
	errorsx.Log(errorsx.Wrap(httpx.WriteJSON(w, httpx.GetBuffer(r), req.Enqueued), "unable to write response"))
}
