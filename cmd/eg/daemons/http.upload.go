package daemons

import (
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
)

// NewUploadHandler constructs the POST /b/upload handler, using the default
// on-disk spool directories.
func NewUploadHandler() *UploadHandler {
	return &UploadHandler{
		Dirs: runners.DefaultSpoolDirs(),
	}
}

// UploadHandler implements POST /b/upload: it accepts a pre-built kernel
// archive + environment file pushed to this runner and enqueues them
// directly (no compile step -- see EnqueueHandler in http.enqueue.go for the
// source-ref equivalent). Dirs is exported so tests can point this at an
// isolated SpoolDirs instead of the process default.
type UploadHandler struct {
	Dirs runners.SpoolDirs
}

func (t *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err           error
		uid           uuid.UUID
		kernelc, envc multipart.File
		kernelh, envh *multipart.FileHeader
	)

	if uid, err = uuid.NewV7(); err != nil {
		log.Println(errorsx.Wrap(err, "unable to generate uuid"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	if kernelc, kernelh, err = r.FormFile("kernel"); err != nil {
		log.Println(errorsx.Wrap(err, "kernel file parameter required"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}
	defer kernelc.Close()

	if err = t.Dirs.Download(uid, kernelh.Filename, kernelc); err != nil {
		log.Println(errorsx.Wrap(err, "unable to receive kernel archive"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	if envc, envh, err = r.FormFile("environ"); err != nil {
		log.Println(errorsx.Wrap(err, "environ file parameter required"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}
	defer envc.Close()

	if err = t.Dirs.Download(uid, envh.Filename, envc); err != nil {
		log.Println(errorsx.Wrap(err, "unable to receive environment file"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	if err = t.Dirs.Enqueue(uid); err != nil {
		log.Println(errorsx.Wrap(err, "unable to enqueue"))
		errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
		return
	}

	log.Println("enqueued", filepath.Join(t.Dirs.Queued, uid.String()))
}
