// Package debrepo manages the deb namespaces of an account. a namespace is an apt repository
// served at {namespace}.{account id}.apt.egdaemon.com whose indexes are signed by the publisher.
// the functions take an http client authorized for the compute api, see authn.New.
package debrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

const (
	// EnvAptHostname overrides the hostname namespaces are served under.
	EnvAptHostname = "EG_DEB_APT_HOSTNAME"
	// EnvToken the token used by publishers, see egdebuild.Publish.
	EnvToken = "EG_DEB_TOKEN"
)

// StatusError the api responded with an error status.
type StatusError struct {
	Code   int
	Status string
	Reason string
}

func (t *StatusError) Error() string {
	if t.Reason != "" {
		return fmt.Sprintf("%s: %s", t.Status, t.Reason)
	}

	return t.Status
}

// NotFound reports whether err is the api responding that the namespace does not exist.
func NotFound(err error) bool {
	var status *StatusError
	return errors.As(err, &status) && status.Code == http.StatusNotFound
}

func endpoint() string {
	return fmt.Sprintf("%s/deb/n", eg.EnvAPIHostDefault())
}

// Hostname the host the namespace is served at.
func Hostname(n *Namespace) string {
	return fmt.Sprintf("%s.%s.%s", n.GetName(), n.GetAccountId(), envx.String("apt.egdaemon.com", EnvAptHostname))
}

// Create the namespace.
func Create(ctx context.Context, c *http.Client, n *Namespace) (*Namespace, error) {
	var out CreateResponse
	if err := do(ctx, c, http.MethodPost, endpoint()+"/", &CreateRequest{Namespace: n}, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to create namespace")
	}

	return out.GetNamespace(), nil
}

// Find the namespace with the name.
func Find(ctx context.Context, c *http.Client, name string) (*Namespace, error) {
	var out FindResponse
	if err := do(ctx, c, http.MethodGet, endpoint()+"/"+url.PathEscape(name), nil, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to find namespace")
	}

	return out.GetNamespace(), nil
}

// Search a single page of namespaces. the response's next carries the offset of the following page,
// zero once there are no more pages.
func Search(ctx context.Context, c *http.Client, offset uint64) (*SearchResponse, error) {
	var out SearchResponse
	q := url.Values{"offset": {strconv.FormatUint(offset, 10)}}
	if err := do(ctx, c, http.MethodGet, endpoint()+"/?"+q.Encode(), nil, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to search namespaces")
	}

	return &out, nil
}

// Update the description and visibility of the namespace, signing keys are replaced when provided.
func Update(ctx context.Context, c *http.Client, name string, n *Namespace) (*Namespace, error) {
	var out UpdateResponse
	if err := do(ctx, c, http.MethodPatch, endpoint()+"/"+url.PathEscape(name), &UpdateRequest{Namespace: n}, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to update namespace")
	}

	return out.GetNamespace(), nil
}

// Remove the namespace with the name, its packages are garbage collected.
func Remove(ctx context.Context, c *http.Client, name string) error {
	return errorsx.Wrap(do(ctx, c, http.MethodDelete, endpoint()+"/"+url.PathEscape(name), nil, nil), "unable to remove namespace")
}

func do(ctx context.Context, c *http.Client, method string, target string, in any, out any) error {
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return errorsx.Wrap(err, "unable to encode request")
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return errorsx.Wrap(err, "unable to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return errorsx.Wrap(err, "request failed")
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		var perr PublishError
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&perr)
		return &StatusError{Code: resp.StatusCode, Status: resp.Status, Reason: perr.GetReason()}
	}

	if out == nil {
		return nil
	}

	return errorsx.Wrap(json.NewDecoder(resp.Body).Decode(out), "unable to decode response")
}
