package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/james-lawrence/eg"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/httpx"
)

func NewRegistrationClient(c *http.Client) *RegistrationClient {
	return &RegistrationClient{
		c:    c,
		host: envx.String("https://localhost:30001", eg.EnvEGAPIHost),
	}
}

type RegistrationClient struct {
	c    *http.Client
	host string
}

func (t RegistrationClient) Register(ctx context.Context, req *RegisterRequest) (_ *RegisterResponse, err error) {
	var (
		encoded []byte
		resp    RegisterResponse
	)

	if encoded, err = json.Marshal(req); err != nil {
		return nil, err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/eg/registration", t.host), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	if err != nil {
		httpresp.Body.Close()
		return nil, err
	}
	defer httpresp.Body.Close()

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
