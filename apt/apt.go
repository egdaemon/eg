// Package apt is the client of the eg apt repository service. repositories are served from
// https://<account id>.<domain>/<repository>, the functions take an http client authorized for
// the account: the ci authorization (see authn.EndpointCompute) for reading, uploading and
// publishing, the meta authorization (see authn.EndpointMetaAuthz) for creating and changing
// repositories.
package apt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

const (
	// Scheme of the apt acquire method that authorizes requests with eg tokens, see Method.
	Scheme = "egs"
	// ContentTypeDeb the content type debian packages are uploaded with.
	ContentTypeDeb = "application/vnd.debian.binary-package"
)

var namerx = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

// ValidName reports an error when the name is not a valid repository name.
func ValidName(name string) error {
	if !namerx.MatchString(name) {
		return fmt.Errorf("invalid repository name '%s', it must match %s", name, namerx.String())
	}

	return nil
}

// Host of the account's repositories.
func Host(domain string, account string) string {
	return fmt.Sprintf("%s.%s", account, domain)
}

// URL of the repository using the scheme, https or egs.
func URL(scheme string, domain string, account string, repo string) string {
	return (&url.URL{Scheme: scheme, Host: Host(domain, account), Path: "/" + repo}).String()
}

// StatusError the repository service responded with an error status.
type StatusError struct {
	Code   int
	Status string
	Body   string
}

func (t *StatusError) Error() string {
	if t.Body == "" {
		return t.Status
	}

	return fmt.Sprintf("%s: %s", t.Status, t.Body)
}

// Status reports whether err is the repository service responding with the status code.
func Status(err error, code int) bool {
	var status *StatusError
	return errors.As(err, &status) && status.Code == code
}

func statuserr(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return &StatusError{Code: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(body))}
}

// Settings of a repository. every field is optional, nil fields are left unchanged by Set and
// take the server defaults on Create. durations are go duration strings, such as "2160h".
type Settings struct {
	Public                 *bool    `json:"public,omitempty"`
	Suite                  *string  `json:"suite,omitempty"`
	Codename               *string  `json:"codename,omitempty"`
	Component              *string  `json:"component,omitempty"`
	Origin                 *string  `json:"origin,omitempty"`
	Label                  *string  `json:"label,omitempty"`
	Architectures          []string `json:"architectures,omitempty"`
	KeepN                  *int     `json:"keepn,omitempty"`
	KeepNGracePeriod       *string  `json:"keepn_grace_period,omitempty"`
	UnpublishedGracePeriod *string  `json:"unpublished_grace_period,omitempty"`
}

// Upload result of a single debian package.
type Upload struct {
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Bytes    int64  `json:"bytes"`
	Staged   bool   `json:"staged"`
}

// Client of a single repository.
type Client struct {
	c    *http.Client
	base string
}

// New client for the repository at base, https://<account id>.<domain>/<repository>.
func New(c *http.Client, base string) Client {
	return Client{c: c, base: strings.TrimSuffix(base, "/")}
}

func (t Client) do(ctx context.Context, method string, path string, body io.Reader, mutate func(*http.Request)) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, t.base+path, body)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create request")
	}

	if mutate != nil {
		mutate(req)
	}

	resp, err := t.c.Do(req)
	if err != nil {
		return nil, errorsx.Wrap(err, "request failed")
	}

	return resp, nil
}

func (t Client) settings(ctx context.Context, method string, s Settings, expected int) (json.RawMessage, error) {
	encoded, err := json.Marshal(s)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to encode settings")
	}

	resp, err := t.do(ctx, method, "", strings.NewReader(string(encoded)), func(r *http.Request) {
		r.Header.Set("Content-Type", "application/json")
	})
	if err != nil {
		return nil, err
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode != expected {
		return nil, statuserr(resp)
	}

	return io.ReadAll(resp.Body)
}

// Create the repository, returns the repository json.
func (t Client) Create(ctx context.Context, s Settings) (json.RawMessage, error) {
	return t.settings(ctx, http.MethodPost, s, http.StatusCreated)
}

// Set changes the settings of the repository, it never republishes. returns the repository json.
func (t Client) Set(ctx context.Context, s Settings) (json.RawMessage, error) {
	return t.settings(ctx, http.MethodPatch, s, http.StatusOK)
}

// Show the repository json.
func (t Client) Show(ctx context.Context) (json.RawMessage, error) {
	resp, err := t.do(ctx, http.MethodGet, "", nil, nil)
	if err != nil {
		return nil, err
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, statuserr(resp)
	}

	return io.ReadAll(resp.Body)
}

// Key writes the armored signing public key of the repository to dst.
func (t Client) Key(ctx context.Context, dst io.Writer) error {
	resp, err := t.do(ctx, http.MethodGet, "/key.asc", nil, nil)
	if err != nil {
		return err
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode != http.StatusOK {
		return statuserr(resp)
	}

	_, err = io.Copy(dst, resp.Body)
	return errorsx.Wrap(err, "unable to read key")
}

// Upload stages the debian package at path. a package identical to the published one is not
// staged. a StatusError with 409 is a different package published at the same pool path, 400 is
// not a valid debian package.
func (t Client) Upload(ctx context.Context, path string) (zero Upload, err error) {
	var out Upload

	src, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return zero, err
	}

	resp, err := t.do(ctx, http.MethodPut, "/upload/"+url.PathEscape(filepath.Base(path)), src, func(r *http.Request) {
		r.ContentLength = info.Size()
		r.Header.Set("Content-Type", ContentTypeDeb)
	})
	if err != nil {
		return zero, err
	}
	defer httpx.AutoClose(resp)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return zero, statuserr(resp)
	}

	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return zero, errorsx.Wrap(err, "unable to decode response")
	}

	return out, nil
}

// Publish the staged packages, returns the published version. a StatusError with 409 is a
// concurrent publish, the server never retries.
func (t Client) Publish(ctx context.Context) (string, error) {
	var out struct {
		Vid json.RawMessage `json:"vid"`
	}

	resp, err := t.do(ctx, http.MethodPost, "/publish", nil, nil)
	if err != nil {
		return "", err
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode != http.StatusOK {
		return "", statuserr(resp)
	}

	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", errorsx.Wrap(err, "unable to decode response")
	}

	// the version is reported as given whether it is a json string or number.
	var s string
	if err := json.Unmarshal(out.Vid, &s); err == nil {
		return s, nil
	}

	return string(out.Vid), nil
}

// PublishRetry publishes, retrying a concurrent publish (409) up to retries times. the wait
// before retry n (starting at 0) is backoff(n).
func (t Client) PublishRetry(ctx context.Context, retries int, backoff func(int) time.Duration) (vid string, err error) {
	for attempt := 0; ; attempt++ {
		if vid, err = t.Publish(ctx); err == nil || !Status(err, http.StatusConflict) || attempt >= retries {
			return vid, err
		}

		select {
		case <-ctx.Done():
			return "", context.Cause(ctx)
		case <-time.After(backoff(attempt)):
		}
	}
}

// Backoff doubles the wait from base on every retry.
func Backoff(base time.Duration) func(int) time.Duration {
	return func(n int) time.Duration {
		return base << n
	}
}
