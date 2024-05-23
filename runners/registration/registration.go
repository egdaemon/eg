package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

func NewRegistrationClient(c *http.Client) *RegistrationClient {
	return &RegistrationClient{
		c:    c,
		host: envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost),
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
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()

	if httpx.IsStatusError(err, http.StatusForbidden) != nil {
		return nil, errorsx.UserFriendly(errorsx.NewUnrecoverable(errorsx.Wrap(err, "enable networking logging export `export EG_LOGS_NETWORK=\"1\"` for more details")))
	} else if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (t RegistrationClient) Search(ctx context.Context, req *RegistrationSearchRequest) (_ *RegistrationSearchResponse, err error) {
	var (
		encoded []byte
		resp    RegistrationSearchResponse
	)

	if encoded, err = json.Marshal(req); err != nil {
		return nil, err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/eg/registration/", t.host), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (t RegistrationClient) Grant(ctx context.Context, req *RegistrationGrantRequest) (_ *RegistrationGrantResponse, err error) {
	var (
		encoded []byte
		resp    RegistrationGrantResponse
	)

	if encoded, err = json.Marshal(req); err != nil {
		return nil, err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/eg/registration/authz", t.host), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
