package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/httpx"
)

func NewRegistrationClient(c *http.Client) *RegistrationClient {
	return &RegistrationClient{
		c:    c,
		host: envx.String("https://localhost:3001", eg.EnvEGAPIHost),
	}
}

type RegistrationClient struct {
	c    *http.Client
	host string
}

func (t RegistrationClient) Registration(ctx context.Context, req *RegistrationRequest) (_ *RegistrationResponse, err error) {
	var (
		encoded []byte
		resp    RegistrationResponse
	)

	if encoded, err = json.Marshal(req); err != nil {
		return nil, err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/eg/registration/", t.host), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	if err != nil {
		if httpresp != nil {
			httpresp.Body.Close()
		}
		return nil, err
	}
	defer httpresp.Body.Close()

	// accepted status means we've received the request but its not yet authorized.
	if httpx.CheckStatusCode(httpresp.StatusCode, http.StatusAccepted) {
		return nil, errorsx.NewTemporary(errors.New("registration not yet authorized"))
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
