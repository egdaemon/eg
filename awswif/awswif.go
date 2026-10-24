// Package awswif manages the aws workload identity federation connections that let runners
// present egdaemon's identity to a customer's aws account. the functions take an http client
// authorized for the meta api, see authn.New with authn.EndpointMetaAuthz.
package awswif

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
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// StatusError the api responded with an error status.
type StatusError struct {
	Code   int
	Status string
}

func (t *StatusError) Error() string {
	return t.Status
}

// NotFound reports whether err is the api responding that the connection does not exist.
func NotFound(err error) bool {
	var status *StatusError
	return errors.As(err, &status) && status.Code == http.StatusNotFound
}

func endpoint() string {
	return fmt.Sprintf("%s/m/aws/wif", eg.EnvAPIHostDefault())
}

// Create the connection. an empty label defaults to the account id, an existing label is overwritten.
func Create(ctx context.Context, c *http.Client, conn *AWSWIFConnection) (*AWSWIFConnection, error) {
	var out AWSWIFCreateResponse
	if err := do(ctx, c, http.MethodPost, endpoint()+"/", &AWSWIFCreateRequest{Connection: conn}, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to create connection")
	}

	return out.GetConnection(), nil
}

// Find the connection with the label.
func Find(ctx context.Context, c *http.Client, label string) (*AWSWIFConnection, error) {
	var out AWSWIFFindResponse
	if err := do(ctx, c, http.MethodGet, endpoint()+"/"+url.PathEscape(label), nil, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to find connection")
	}

	return out.GetConnection(), nil
}

// Search a single page of connections whose label, role arn, or description contain query. the response's
// next carries the offset of the following page, zero once there are no more pages.
func Search(ctx context.Context, c *http.Client, query string, offset uint64) (*AWSWIFSearchResponse, error) {
	var out AWSWIFSearchResponse
	q := url.Values{"offset": {strconv.FormatUint(offset, 10)}, "query": {query}}
	if err := do(ctx, c, http.MethodGet, endpoint()+"/?"+q.Encode(), nil, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to search connections")
	}

	return &out, nil
}

// Update the connection, only the fields set on req are changed.
func Update(ctx context.Context, c *http.Client, label string, req *AWSWIFUpdateRequest) (*AWSWIFConnection, error) {
	var out AWSWIFUpdateResponse
	if err := do(ctx, c, http.MethodPatch, endpoint()+"/"+url.PathEscape(label), req, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to update connection")
	}

	return out.GetConnection(), nil
}

// Remove the connection with the label.
func Remove(ctx context.Context, c *http.Client, label string) error {
	return errorsx.Wrap(do(ctx, c, http.MethodDelete, endpoint()+"/"+url.PathEscape(label), nil, nil), "unable to remove connection")
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
		return &StatusError{Code: resp.StatusCode, Status: resp.Status}
	}

	if out == nil {
		return nil
	}

	return errorsx.Wrap(json.NewDecoder(resp.Body).Decode(out), "unable to decode response")
}

// Issuer retrieves what the trust policy of an aws account needs to trust the workloads of the account: the url of
// the issuer to register as the account's OIDC identity provider and the subject of its identities.
func Issuer(ctx context.Context, c *http.Client) (*AWSWIFIssuerResponse, error) {
	var out AWSWIFIssuerResponse
	if err := do(ctx, c, http.MethodGet, fmt.Sprintf("%s/m/aws/oidc/", eg.EnvAPIHostDefault()), nil, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve issuer")
	}

	return &out, nil
}
