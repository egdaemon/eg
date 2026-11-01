// Package me manages the authenticated profile itself. the functions take an http client
// authorized for the meta api, see authn.New with authn.EndpointMetaAuthz.
package me

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
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

// BadRequest reports whether err is the api rejecting the request, for example an invalid email.
func BadRequest(err error) bool {
	var status *StatusError
	return errors.As(err, &status) && status.Code == http.StatusBadRequest
}

func endpoint() string {
	return fmt.Sprintf("%s/me", eg.EnvAPIHostDefault())
}

// UpdateProfile updates the profile of the token, the profile is never part of the request.
// the display and email replace the stored values, they are sent as given.
func UpdateProfile(ctx context.Context, c *http.Client, display string, email string) (*authn.Profile, error) {
	var out MeProfileUpdateResponse

	encoded, err := json.Marshal(&MeProfileUpdateRequest{Profile: &authn.Profile{Display: display, Email: email}})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to encode request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint()+"/", bytes.NewReader(encoded))
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, errorsx.Wrap(err, "request failed")
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errorsx.Wrap(&StatusError{Code: resp.StatusCode, Status: resp.Status}, "unable to update profile")
	}

	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode response")
	}

	return out.GetProfile(), nil
}
