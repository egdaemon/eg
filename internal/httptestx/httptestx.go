package httptestx

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

func BuildURL(path string, v url.Values) string {
	return (&url.URL{
		Scheme:   "http",
		Host:     "example.com",
		Path:     path,
		RawQuery: v.Encode(),
	}).String()
}

// ReadRequest reads a request from a file.
func ReadRequest(path string) (resp *httptest.ResponseRecorder, req *http.Request, err error) {
	var (
		raw []byte
	)

	if raw, err = os.ReadFile(path); err != nil {
		return nil, nil, err
	}

	req, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(raw)))
	return httptest.NewRecorder(), req, err
}

// BuildRequest ...
func BuildRequest(method string, uri string, body []byte) (*httptest.ResponseRecorder, *http.Request, error) {
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(strings.ToUpper(method), uri, bytes.NewBuffer(body))
	return recorder, req, err
}

// BuildGetRequest ...
func BuildGetRequest(body []byte) (*httptest.ResponseRecorder, *http.Request, error) {
	return BuildRequest(http.MethodGet, "http://example.com", body)
}

// BuildPostRequest ...
func BuildPostRequest(body []byte) (*httptest.ResponseRecorder, *http.Request, error) {
	return BuildRequest(http.MethodPost, "http://example.com", body)
}

// BuildDeleteRequest ...
func BuildDeleteRequest(body []byte) (*httptest.ResponseRecorder, *http.Request, error) {
	return BuildRequest(http.MethodDelete, "http://example.com", body)
}

// BuildWebsocketConn ...
func BuildWebsocketConn() (server *httptest.Server, cconn *websocket.Conn, sconn *websocket.Conn, resp *http.Response, err error) {
	out := make(chan *websocket.Conn, 1)
	errs := make(chan error)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var (
			nconn      *websocket.Conn
			upgradeErr error
		)

		nconn, upgradeErr = (&websocket.Upgrader{}).Upgrade(w, req, nil)
		if upgradeErr != nil {
			errs <- err
			return
		}
		out <- nconn
	})
	server = httptest.NewServer(mux)

	tallachatURL, err := url.Parse(fmt.Sprintf("ws://%s", server.Listener.Addr()))
	if err != nil {
		return server, cconn, sconn, resp, err
	}
	cconn, resp, err = websocket.DefaultDialer.Dial(tallachatURL.String(), nil)
	select {
	case sconn = <-out:
	case err = <-errs:
	}
	return server, cconn, sconn, resp, err
}

// RoundTripFunc ...
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip ...
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}
