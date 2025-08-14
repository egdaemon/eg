package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/time/rate"
)

// CheckStatusCode compares the provided status code with a list of acceptable
// status codes.
func CheckStatusCode(actual int, acceptable ...int) bool {
	for _, code := range acceptable {
		if actual == code {
			return true
		}
	}

	return false
}

// IsSuccess - returns true iff the response code was one of the following:
// http.StatusOK, http.StatusAccepted, http.StatusCreated. Delegates to CheckStatusCode, http.StatusNoContent.
func IsSuccess(actual int) bool {
	return CheckStatusCode(actual, http.StatusNoContent, http.StatusOK, http.StatusAccepted, http.StatusCreated)
}

// Get return a get request for the given endpoint
func Get(ctx context.Context, endpoint string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, endpoint, strings.NewReader(""))
}

// ParseFormHandler automatically triggers a parse of the request form.
func ParseFormHandler(original http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			http.Error(resp, "malformatted form", http.StatusBadRequest)
			return
		}

		original.ServeHTTP(resp, req)
	})
}

// RouteInvoked wraps a http.Handler and emits route invocations.
func RouteInvoked(original http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		p := req.Host + req.URL.Path
		started := time.Now()
		log.Println(p, "initiated")
		original.ServeHTTP(resp, req)
		log.Println(p, "completed", time.Since(started))
	})
}

// RouteRateLimited applies a rate limit to the http handler.
func RouteRateLimited(l *rate.Limiter) func(http.Handler) http.Handler {
	return func(original http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if l.Allow() {
				original.ServeHTTP(resp, req)
				return
			}

			resp.Header().Add("Retry-After", fmt.Sprintf("%d", int(time.Second)))
			resp.WriteHeader(http.StatusTooManyRequests)
		})
	}
}

// Debug dumps the request to STDERR.
func Debug(original http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		raw, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Println(errorsx.Wrap(err, "failed to dump request"))
		} else {
			log.Println(string(raw))
		}
		original.ServeHTTP(resp, req)
	})
}

// RecordRequestHandler records the request to a temporary file.
// does not clean up the file from disk.
func RecordRequestHandler(original http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		var (
			err error
			raw []byte
			out *os.File
		)

		if raw, err = httputil.DumpRequest(req, true); err != nil {
			log.Println("failed to dump request", err)
			goto next
		}

		if out, err = os.CreateTemp("", "request-recording"); err != nil {
			log.Println("failed to record request", err)
			goto next
		}
		defer out.Close()

		if _, err = out.Write(raw); err != nil {
			log.Println("failed to record contents to file", err)
			goto next
		}
	next:
		original.ServeHTTP(resp, req)
	})
}

// HTTPRequestScheme return the http scheme for a request.
func HTTPRequestScheme(req *http.Request) string {
	const (
		scheme       = "http"
		secureScheme = "https"
	)

	if req.TLS == nil {
		return scheme
	}

	return secureScheme
}

// WebsocketRequestScheme return the websocket scheme for a request.
func WebsocketRequestScheme(req *http.Request) string {
	const (
		scheme       = "ws"
		secureScheme = "wss"
	)

	if req.TLS == nil {
		return scheme
	}

	return secureScheme
}

// WriteJSON writes a json payload into the provided buffer and sets the context-type header to application/json.
func WriteJSON(resp http.ResponseWriter, buffer *bytes.Buffer, context interface{}) error {
	var (
		err error
	)

	buffer.Reset()
	resp.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(buffer).Encode(context); err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return err
	}

	_, err = io.Copy(resp, buffer)
	return err
}

// WriteEmptyJSONArray emits an empty json array with the provided status code.
func WriteEmptyJSONArray(resp http.ResponseWriter, code int) {
	const emptyJSON = "[]"
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(code)
	if _, err := resp.Write([]byte(emptyJSON)); err != nil {
		log.Println("unable to write response", err)
	}
}

// WriteEmptyJSON emits empty json with the provided status code.
func WriteEmptyJSON(resp http.ResponseWriter, code int) error {
	const emptyJSON = "{}"
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(code)
	if _, err := resp.Write([]byte(emptyJSON)); err != nil {
		return errorsx.Wrap(err, "unable to write response")
	}

	return nil
}

// RedirectHTTPRequest generates a url to redirect to from the provided
// request and destination node
func RedirectHTTPRequest(req *http.Request, dst string, defaultPort string) *url.URL {
	_, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		debugx.Println("using default port error splitting request host", err)
		port = defaultPort
	}

	return &url.URL{
		Scheme:   HTTPRequestScheme(req),
		Host:     net.JoinHostPort(dst, port),
		Path:     req.URL.Path,
		RawQuery: req.URL.Query().Encode(),
	}
}

// ErrorCode ...
func ErrorCode(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	return Error{Code: resp.StatusCode, cause: errorsx.New(resp.Status)}
}

func AsError(r *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return r, err
	}

	if r.StatusCode >= 400 {
		return r, &Error{Code: r.StatusCode, cause: errorsx.New(r.Status)}
	}

	return r, nil
}

func AutoClose(r *http.Response) error {
	if r == nil {
		return nil
	}

	return r.Body.Close()
}

// Error ...
type Error struct {
	Code  int
	cause error
}

func (t Error) Error() string {
	return t.cause.Error()
}

func (t Error) Is(target error) bool {
	_, ok := target.(*Error)
	return ok
}

func (t Error) As(target any) bool {
	if x, ok := target.(*Error); ok {
		*x = t
		return ok
	}

	return false
}

// IgnoreError ...
func IgnoreError(err error, code ...int) bool {
	var (
		cause Error
		ok    bool
	)

	if cause, ok = errorsx.Cause(err).(Error); !ok {
		return false
	}

	return CheckStatusCode(cause.Code, code...)
}

func IsStatusError(err error, code ...int) error {
	var (
		cause Error
	)

	if !errors.As(err, &cause) {
		return nil
	}

	if !CheckStatusCode(cause.Code, code...) {
		return nil
	}

	return err
}

// MimeType extracts mimetype from request, defaults to application/
func MimeType(h http.Header) string {
	const fallback = "application/octet-stream"
	t, _, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		return fallback
	}

	return stringsx.DefaultIfBlank(t, fallback)
}

// Notfound response handler
func NotFound(resp http.ResponseWriter, req *http.Request) {
	raw, _ := httputil.DumpRequest(req, false)
	log.Println("requested endpoint not found", string(raw))
	resp.WriteHeader(http.StatusNotFound)
}

func Multipart(do func(*multipart.Writer) error) (contentType string, _ io.ReadCloser, err error) {
	r, w := io.Pipe()

	mw := multipart.NewWriter(w)

	go func() {
		errorsx.Log(w.CloseWithError(errorsx.Compact(do(mw), mw.Close())))
	}()

	return mw.FormDataContentType(), r, nil
}

func escapeQuotes(s string) string {
	quoteEscaper := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
	return quoteEscaper.Replace(s)
}

func NewMultipartHeader(mimetype string, fieldname string, filename string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", mimetype)
	return h
}

// TryClose attempts to close the response body if it exists.
func TryClose(r *http.Response) error {
	if r == nil {
		return nil
	}

	return r.Body.Close()
}
