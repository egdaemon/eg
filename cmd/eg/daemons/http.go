package daemons

import (
	"database/sql"
	"log"
	"net"
	"net/http"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/historyapi"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runners"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

// uploadable gates the runner's push HTTP surface (POST /c/upload, POST /c/enqueue, GET /c/logs/{id}, GET /c/history/{id}).
func HTTP(global *cmdopts.Global, httpl net.Listener, rm *runners.ResourceManager, compiledirs runners.SpoolDirs, reserver Reserver, db *sql.DB, uploadable bool) (err error) {
	httpmux := mux.NewRouter()
	httpmux.NotFoundHandler = alice.New(httpx.RouteInvoked).ThenFunc(httpx.NotFound)

	httpmux.HandleFunc("/healthz", httpx.Healthz(envx.Int(http.StatusOK, cmdopts.EnvHealthzCode))).Methods("GET")

	// gates the runner's push HTTP surface (POST /c/upload, POST /c/enqueue, GET /c/logs/{id}, GET /c/history/{id}).
	apigate := httpx.GatedResponse(uploadable, http.StatusForbidden)

	// POST /c/upload accepts a pre-built kernel archive + environment file
	// pushed to this runner, records it with the control plane, and enqueues
	// it directly, subject to current load. See http.upload.go for the
	// (independently testable) handler implementation.
	upload := NewUploadHandler(reserver)
	upload.RM = rm
	httpmux.Handle("/c/upload", alice.New(httpx.RouteInvoked, apigate).Then(upload)).Methods(http.MethodPost)

	// POST /c/enqueue pushes a source-ref submission (instead of a pre-built
	// archive) to this runner: the runner clones and compiles it itself,
	// asynchronously, after deciding synchronously whether to admit it based
	// on current load. See http.enqueue.go for the (independently testable)
	// handler implementation.
	httpmux.Handle("/c/enqueue", alice.New(httpx.RouteInvoked, apigate).Then(NewEnqueueHandler(compiledirs, rm))).Methods(http.MethodPost)

	// GET /c/logs/{id} streams a workload's log as it is written (tail -f),
	// authorized by a log token signed with this runner's secret. See
	// http.logs.go for the (independently testable) handler implementation.
	httpmux.Handle("/c/logs/{id}", alice.New(httpx.RouteInvoked, apigate).Then(NewLogsHandler())).Methods(http.MethodGet)

	// GET /c/history/{id} and GET /c/history/{id}/logs report the recorded result and logs of a workload this
	// runner completed, authorized by the same log token. See historyapi for the (independently testable) implementation.
	historyr := httpmux.PathPrefix("/c/history").Subrouter()
	historyr.Use(apigate)
	historyapi.NewHTTPHistory(db, JWTSecret()).Bind(historyr)

	global.Cleanup.Go(func() {
		defer global.Shutdown(nil)
		defer log.Println("http shutting down")
		if err := http.Serve(httpl, httpmux); err != nil {
			log.Println("failed to start http server", err)
		}
	})

	return nil
}
