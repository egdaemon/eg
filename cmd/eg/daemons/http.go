package daemons

import (
	"log"
	"net"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runners"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

func HTTP(global *cmdopts.Global, httpl net.Listener, rm *runners.ResourceManager, compiledirs runners.SpoolDirs) (err error) {
	httpmux := mux.NewRouter()
	httpmux.NotFoundHandler = alice.New(httpx.RouteInvoked).ThenFunc(httpx.NotFound)

	httpmux.HandleFunc("/healthz", httpx.Healthz(envx.Int(http.StatusOK, cmdopts.EnvHealthzCode))).Methods("GET")

	// gates the runner's push HTTP surface (POST /b/upload, POST /c/enqueue) as a
	// stopgap ahead of real request authentication -- see the accompanying plan doc.
	apigate := httpx.GatedResponse(envx.Boolean(false, eg.EnvComputeAPIEnabled), http.StatusForbidden)

	// POST /b/upload accepts a pre-built kernel archive + environment file
	// pushed to this runner and enqueues them directly. See http.upload.go
	// for the (independently testable) handler implementation.
	httpmux.Handle("/b/upload", alice.New(httpx.RouteInvoked, apigate).Then(NewUploadHandler())).Methods(http.MethodPost)

	// POST /c/enqueue pushes a source-ref submission (instead of a pre-built
	// archive) to this runner: the runner clones and compiles it itself,
	// asynchronously, after deciding synchronously whether to admit it based
	// on current load. See http.enqueue.go for the (independently testable)
	// handler implementation.
	httpmux.Handle("/c/enqueue", alice.New(httpx.RouteInvoked, apigate).Then(NewEnqueueHandler(compiledirs, rm))).Methods(http.MethodPost)

	global.Cleanup.Go(func() {
		defer global.Shutdown(nil)
		defer log.Println("http shutting down")
		if err := http.Serve(httpl, httpmux); err != nil {
			log.Println("failed to start http server", err)
		}
	})

	return nil
}
