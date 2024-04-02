package daemons

import (
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

func HTTP(global *cmdopts.Global, httpl net.Listener) (err error) {
	httpmux := mux.NewRouter()
	httpmux.NotFoundHandler = alice.New(httpx.RouteInvoked).ThenFunc(httpx.NotFound)

	httpmux.HandleFunc("/healthz", httpx.Healthz(envx.Int(http.StatusOK, cmdopts.EnvHealthzCode))).Methods("GET")

	httpmux.Handle("/b/upload", alice.New(httpx.RouteInvoked).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			err           error
			uid           uuid.UUID
			kernelc, envc multipart.File
			kernelh, envh *multipart.FileHeader
		)

		dirs := runners.DefaultSpoolDirs()

		if uid, err = uuid.NewV7(); err != nil {
			log.Println(errorsx.Wrap(err, "unable to generate uuid"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}

		if kernelc, kernelh, err = r.FormFile("kernel"); err != nil {
			log.Println(errorsx.Wrap(err, "kernel file parameter required"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}
		defer kernelc.Close()

		if err = dirs.Download(uid, kernelh.Filename, kernelc); err != nil {
			log.Println(errorsx.Wrap(err, "unable to receive kernel archive"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}

		if envc, envh, err = r.FormFile("environ"); err != nil {
			log.Println(errorsx.Wrap(err, "environ file parameter required"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}
		defer envc.Close()

		if err = dirs.Download(uid, envh.Filename, envc); err != nil {
			log.Println(errorsx.Wrap(err, "unable to receive environment file"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}

		if err = dirs.Enqueue(uid); err != nil {
			log.Println(errorsx.Wrap(err, "unable to enqueue"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}

		log.Println("enqueued", filepath.Join(dirs.Queued, uid.String()))
	})).Methods("POST")

	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()
		defer global.Shutdown()
		if err := http.Serve(httpl, httpmux); err != nil {
			log.Println("failed to start http server", err)
		}
	}()

	return nil
}
