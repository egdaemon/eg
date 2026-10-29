package daemons

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/egdaemon/eg/jsonx"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
)

// Reserver records a workload uploaded directly to this runner with the
// control plane, using the candidate token the client presented, returning
// the recorded workload.
type Reserver interface {
	Reserve(ctx context.Context, enq *computeapi.Enqueued, archive io.Reader, token string) (*computeapi.EnqueuedDequeueResponse, error)
}

// NewUploadHandler constructs the POST /c/upload handler, using the default
// on-disk spool directories.
func NewUploadHandler(r Reserver) *UploadHandler {
	return &UploadHandler{
		Dirs:     runners.DefaultSpoolDirs(),
		Reserver: r,
	}
}

// UploadHandler implements POST /c/upload: it accepts a pre-built kernel
// archive pushed to this runner -- an "enqueued" field (JSON-encoded
// computeapi.EnqueuedDequeueResponse), the "kernel" archive and an "environ"
// file -- and enqueues them directly (no compile step -- see EnqueueHandler
// in http.enqueue.go for the source-ref equivalent). the enqueued payload is
// the workload as described by the client, and the "token" field is the
// candidate token egmeta issued for this runner; the workload is recorded
// with egmeta (see Reserver) before it is accepted, and spooled as recorded.
// Dirs/RM are exported so tests can point this at an isolated SpoolDirs and
// ResourceManager instead of the process defaults; a nil RM admits everything.
type UploadHandler struct {
	Dirs     runners.SpoolDirs
	RM       *runners.ResourceManager
	Reserver Reserver
}

func (t *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err           error
		uid           uuid.UUID
		req           computeapi.EnqueuedDequeueResponse
		recorded      *computeapi.EnqueuedDequeueResponse
		encoded       []byte
		kernelc, envc multipart.File
	)

	if err = jsonx.Unmarshal([]byte(r.FormValue("enqueued")), &req); err != nil {
		log.Println(errorsx.Wrap(err, "unable to decode upload request"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	if req.Enqueued == nil {
		log.Println("upload request missing enqueued payload")
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	candidate := r.FormValue("token")
	if stringsx.Blank(candidate) {
		log.Println("upload request missing candidate token")
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	want := runners.RuntimeResources{Cores: req.Enqueued.Cores, Memory: req.Enqueued.Memory, Vram: req.Enqueued.Vram}
	if t.RM != nil && !t.RM.Admit(want) {
		log.Printf("rejecting upload request, insufficient capacity %s available=%+v requested=%+v limit=%+v\n", req.Enqueued.VcsUri, t.RM.Available(), want, t.RM.Limit)
		w.Header().Set("Retry-After", "5") // TODO: long term base this on load.
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusServiceUnavailable))
		return
	}

	if kernelc, _, err = r.FormFile("kernel"); err != nil {
		log.Println(errorsx.Wrap(err, "kernel file parameter required"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}
	defer kernelc.Close()

	if envc, _, err = r.FormFile("environ"); err != nil {
		log.Println(errorsx.Wrap(err, "environ file parameter required"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}
	defer envc.Close()

	// record the workload with the control plane before accepting it; when it
	// cannot be recorded the client moves on to the next candidate or the cluster.
	if recorded, err = t.Reserver.Reserve(r.Context(), req.Enqueued, kernelc, candidate); err != nil {
		log.Println(errorsx.Wrap(err, "unable to record workload with the control plane"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadGateway))
		return
	}

	// from here on the workload is claimed by this runner within the control
	// plane; if we fail to spool it the record is abandoned and expires via its ttl
	// while the client falls back to the cluster.
	if uid, err = uuid.FromString(recorded.Enqueued.Id); err != nil {
		log.Println(errorsx.Wrap(err, "invalid recorded workload, malformed id"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if _, err = kernelc.Seek(0, io.SeekStart); err != nil {
		log.Println(errorsx.Wrap(err, "unable to rewind kernel archive"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if encoded, err = json.Marshal(recorded); err != nil {
		log.Println(errorsx.Wrap(err, "unable to encode recorded workload"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Download(uid, "archive.tar.gz", kernelc); err != nil {
		log.Println(errorsx.Wrap(err, "unable to receive kernel archive"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Download(uid, eg.EnvironFile, envc); err != nil {
		log.Println(errorsx.Wrap(err, "unable to receive environment file"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Download(uid, "metadata.json", bytes.NewReader(encoded)); err != nil {
		log.Println(errorsx.Wrap(err, "unable to persist recorded workload"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	if err = t.Dirs.Enqueue(uid); err != nil {
		log.Println(errorsx.Wrap(err, "unable to enqueue"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusInternalServerError))
		return
	}

	log.Println("enqueued", req.Enqueued.Id, "->", filepath.Join(t.Dirs.Queued, uid.String()))

	w.WriteHeader(http.StatusAccepted)
	errorsx.Log(errorsx.Wrap(httpx.WriteJSON(w, httpx.GetBuffer(r), recorded.Enqueued), "unable to write response"))
}
