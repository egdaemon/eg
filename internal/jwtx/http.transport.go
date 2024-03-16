package jwtx

import (
	"net/http"
)

type signer interface {
	Token() (string, error)
}

type SignerFn func() (string, error)

func (t SignerFn) Token() (string, error) {
	return t()
}

func NewHTTP(c *http.Client, s signer) *http.Client {
	d := c.Transport
	if d == nil {
		d = http.DefaultTransport
	}

	c.Transport = HTTPTransport{
		s: s,
		d: d,
	}

	return c
}

type HTTPTransport struct {
	s signer
	d http.RoundTripper
}

func (t HTTPTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	token, err := t.s.Token()
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", token)

	return t.d.RoundTrip(req)
}
