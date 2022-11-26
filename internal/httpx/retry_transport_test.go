package httpx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/james-lawrence/eg/internal/httptestx"

	. "github.com/james-lawrence/eg/internal/httpx"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retry Transport", func() {
	It("should retry once", func() {
		invoked := 0
		body := []byte("")
		c := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			body, _ = io.ReadAll(req.Body)
			invoked++

			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}
		})

		c.Transport = NewRetryTransport(c.Transport, http.StatusBadGateway)
		req, err := http.NewRequest(http.MethodGet, "http://example.com/", strings.NewReader("Hello World"))
		Expect(err).To(Succeed())
		resp, err := c.Do(req)
		Expect(err).To(Succeed())
		Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
		Expect(invoked).To(Equal(2))
		Expect(body).To(Equal([]byte("Hello World")))
	})

	It("should retry on context.DeadlineExceeded", func() {
		s := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			time.Sleep(time.Hour)
		}))
		defer s.Close()
		c := http.DefaultClient
		c.Transport = NewRetryTransport(c.Transport, http.StatusBadGateway)

		ctx, done := context.WithTimeout(context.Background(), -1*time.Second)
		defer done()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, strings.NewReader("Hello World"))
		Expect(err).To(Succeed())
		resp, err := c.Do(req)
		Expect(err).ToNot(Succeed())
		Expect(resp).To(BeNil())
	})

	It("should retry with a nil body", func() {
		invoked := 0
		body := []byte("")
		c := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			body, _ = io.ReadAll(req.Body)
			invoked++

			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}
		})

		c.Transport = NewRetryTransport(c.Transport, http.StatusBadGateway)
		req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		Expect(err).To(Succeed())
		resp, err := c.Do(req)
		Expect(err).To(Succeed())
		Expect(resp.StatusCode).To(Equal(http.StatusBadGateway))
		Expect(invoked).To(Equal(2))
		Expect(body).To(Equal([]byte("")))
	})
})
