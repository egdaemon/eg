package daemons

import (
	"crypto/md5"
	"hash"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/httpx"
	"github.com/james-lawrence/eg/internal/md5x"
	"github.com/justinas/alice"
	"github.com/pkg/errors"
)

func HTTP(global *cmdopts.Global, httpl net.Listener) (err error) {

	httpmux := mux.NewRouter()
	httpmux.NotFoundHandler = alice.New(httpx.RouteInvoked).ThenFunc(httpx.NotFound)

	httpmux.HandleFunc("/healthz", httpx.Healthz(envx.Int(http.StatusOK, cmdopts.HealthzCode))).Methods("GET")

	httpmux.Handle("/b/upload", alice.New(httpx.RouteInvoked).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			err           error
			kernelc, envc multipart.File
			kernelh, envh *multipart.FileHeader
			kerneldigest  hash.Hash = md5.New()
			envdigest     hash.Hash = md5.New()
		)
		if kernelc, kernelh, err = r.FormFile("kernel"); err != nil {
			log.Println(errors.Wrap(err, "kernel file parameter required"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}
		defer kernelc.Close()

		if _, err = io.Copy(kerneldigest, kernelc); err != nil {
			log.Println(errors.Wrap(err, "unable to process kernel file"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}

		log.Println("entry point", r.Form.Get("entrypoint"))
		log.Println("minimum cores", r.Form.Get("cores"))
		log.Println("minimum memory", r.Form.Get("memory"))

		if envc, envh, err = r.FormFile("environ"); err != nil {
			log.Println(errors.Wrap(err, "environ file parameter required"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}
		defer envc.Close()

		if _, err = io.Copy(envdigest, envc); err != nil {
			log.Println(errors.Wrap(err, "unable to process environ file"))
			errorsx.MaybeLog(httpx.WriteEmptyJSON(w, http.StatusBadRequest))
			return
		}

		log.Println("kernel file received", kernelh.Filename, kernelh.Header.Get("Content-Type"), md5x.FormatString(kerneldigest))
		log.Println("environ file received", envh.Filename, envh.Header.Get("Content-Type"), md5x.FormatString(envdigest))
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
