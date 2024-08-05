package httpx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httputil"

	"golang.org/x/time/rate"
)

// DebugTransportOption (DTO) debugging transport, prints out the request and response
// to the provided
type DebugTransportOption func(*DebugTransport)

// DTORoundTripper override the default http.RoundTripper to delegate the request
// to. By default uses http.DefaultTransport.
func DTORoundTripper(rt http.RoundTripper) DebugTransportOption {
	return func(dt *DebugTransport) {
		if rt == nil {
			return
		}

		dt.delegate = rt
	}
}

// NewDebugTransport builds a http.RoundTripper that prints the request
// to the standard logger.
func NewDebugTransport(options ...DebugTransportOption) DebugTransport {
	t := DebugTransport{
		delegate: http.DefaultTransport,
	}

	for _, opt := range options {
		opt(&t)
	}

	return t
}

// DebugTransport - prints the request and response of an http request.
type DebugTransport struct {
	delegate http.RoundTripper
}

// RoundTrip - implements http.RoundTripper
func (t DebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		raw  []byte
		err  error
		resp *http.Response
	)

	if raw, err = httputil.DumpRequest(req, true); err == nil {
		log.Println("RAW REQUEST")
		log.Println("Scheme:", req.URL.Scheme)
		log.Println(string(raw))
	}

	resp, err = t.delegate.RoundTrip(req)

	if resp != nil && resp.Body != nil {
		if raw, err = httputil.DumpResponse(resp, true); err != nil {
			return resp, err
		}
		log.Println("RAW RESPONSE")
		log.Println(string(raw))
	}

	return resp, err
}

// DebugClient wraps the client's transport with in a debugger.
func DebugClient(c *http.Client) *http.Client {
	c.Transport = NewDebugTransport(DTORoundTripper(c.Transport))
	return c
}

// HeadersTransportOption (HTO)
type HeadersTransportOption func(*HeadersTransport)

// HTORoundTripper override the default http.RoundTripper to delegate the request
// to. By default uses http.DefaultTransport.
func HTORoundTripper(rt http.RoundTripper) HeadersTransportOption {
	return func(t *HeadersTransport) {
		if rt == nil {
			return
		}

		t.Delegate = rt
	}
}

// NewHeadersTransport builds a transport that adds additional headers.
func NewHeadersTransport(headers http.Header, options ...HeadersTransportOption) HeadersTransport {
	t := HeadersTransport{
		Header:   headers,
		Delegate: http.DefaultTransport,
	}

	for _, opt := range options {
		opt(&t)
	}

	return t
}

// HeadersTransport adds additional headers to a request.
type HeadersTransport struct {
	http.Header
	Delegate http.RoundTripper
}

// RoundTrip - implements http.RoundTripper
func (t HeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, values := range t.Header {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	return t.Delegate.RoundTrip(req)
}

// RateLimitTransportOption options for the RateLimitTransport
type RateLimitTransportOption func(*RateLimitTransport)

// RLTOptionLimiter sets the rate limit for the transport.
func RLTOptionLimiter(l *rate.Limiter) RateLimitTransportOption {
	return func(t *RateLimitTransport) {
		t.Limiter = l
	}
}

// RLTOptionTransport sets the delegate transport for the RateLimitTransport.
func RLTOptionTransport(rt http.RoundTripper) RateLimitTransportOption {
	return func(t *RateLimitTransport) {
		t.Delegate = rt
	}
}

// NewRateLimitTransport creates transport that is capable of adjusting the rate limit of requests.
// defaults to an unlimited rate.
func NewRateLimitTransport(options ...RateLimitTransportOption) (transport RateLimitTransport) {
	transport = RateLimitTransport{
		Limiter:  rate.NewLimiter(rate.Inf, 0),
		Delegate: http.DefaultTransport,
	}

	for _, opt := range options {
		opt(&transport)
	}

	return transport
}

// RateLimitTransport transport that limits the rate at which requests are made.
type RateLimitTransport struct {
	*rate.Limiter
	Delegate http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (t RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.Limiter.Wait(context.Background()); err != nil {
		return nil, err
	}

	return t.Delegate.RoundTrip(req)
}

func BindRetryTransport(c *http.Client, codes ...int) *http.Client {
	c.Transport = NewRetryTransport(c.Transport, codes...)
	return c
}

// NewRetryTransport create a Transport that reattempts a single time if the specified
// codes are seen.
func NewRetryTransport(rt http.RoundTripper, codes ...int) RetryTransport {
	if rt == nil {
		rt = http.DefaultTransport
	}

	m := make(map[int]struct{}, len(codes))
	for _, code := range codes {
		m[code] = struct{}{}
	}

	return RetryTransport{
		pool:     NewBufferPool(1024),
		codes:    m,
		Delegate: rt,
	}
}

// RetryTransport reattempts once on the specified status codes.
type RetryTransport struct {
	pool     BufferPool
	codes    map[int]struct{}
	Delegate http.RoundTripper
}

// RoundTrip - implements http.RoundTripper
func (t RetryTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Body == nil {
		req.Body = io.NopCloser(bytes.NewBufferString(""))
	}

	o := req.Body
	if o != nil {
		defer o.Close()
	}

	buf := bytes.NewBuffer(t.pool.Get())
	tee := io.NopCloser(io.TeeReader(req.Body, buf))
	req.Body = tee

	if resp, err = t.Delegate.RoundTrip(req); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			req.Body = io.NopCloser(buf)
			return t.Delegate.RoundTrip(req)
		}
		return resp, err
	}

	// retry once.
	if _, ok := t.codes[resp.StatusCode]; !ok {
		return resp, err
	}

	req.Body = io.NopCloser(buf)
	return t.Delegate.RoundTrip(req)
}
