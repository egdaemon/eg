package daemons

import (
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/httpx"
	"github.com/justinas/alice"
)

func HTTP(global *cmdopts.Global) (httpl net.Listener, err error) {
	if httpl, err = net.Listen("tcp", "127.0.1.1:8093"); err != nil {
		return nil, err
	}

	httpmux := mux.NewRouter()
	httpmux.NotFoundHandler = alice.New(httpx.RouteInvoked).ThenFunc(httpx.NotFound)

	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()
		defer global.Shutdown()
		if err := http.Serve(httpl, httpmux); err != nil {
			log.Println("failed to start http server", err)
		}
	}()

	return httpl, err
}