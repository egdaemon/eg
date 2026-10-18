// Package cmdgcp manages the gcp workload identity federation connections that let
// runners present egdaemon's identity to a customer's gcp project.
package cmdgcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Cmd struct {
	WIF WIF `cmd:"" name:"wif" help:"manage workload identity federation connections"`
}

type WIF struct {
	Create Create `cmd:"" name:"create" help:"create a connection"`
	List   List   `cmd:"" name:"ls" help:"list connections"`
	Edit   Edit   `cmd:"" name:"edit" help:"edit a connection"`
	Remove Remove `cmd:"" name:"rm" help:"remove a connection"`
}

// client talks to /m/gcp/wif authenticated as the ssh key's profile. the management routes require
// the user management permission.
type client struct {
	http     *http.Client
	endpoint string
	// accountID the account of the authenticated profile, the default label.
	accountID string
}

func newClient(ctx context.Context, gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, sshkeypath string) (*client, error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), sshkeypath)
	if err != nil {
		return nil, err
	}

	return newClientFrom(ctx, tlsc.DefaultClient(), signer, gctx.AccountID)
}

func newClientFrom(ctx context.Context, c *http.Client, signer ssh.Signer, account string) (*client, error) {
	tokensrc := oauth2.ReuseTokenSource(nil, compute.NewAuthzTokenSource(c, signer, authn.EndpointMetaAuthz(), account))
	tok, err := tokensrc.Token()
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to authorize")
	}

	// the signature is verified by the server, we only need the account to default the label.
	var claims jwt.RegisteredClaims
	if _, _, err = jwt.NewParser().ParseUnverified(tok.AccessToken, &claims); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode authorization")
	}

	return &client{
		http:      oauth2.NewClient(context.WithValue(ctx, oauth2.HTTPClient, c), tokensrc),
		endpoint:  fmt.Sprintf("%s/m/gcp/wif", eg.EnvAPIHostDefault()),
		accountID: claims.Issuer,
	}, nil
}

// label the explicit label or the account id of the current profile.
func (t *client) label(explicit string) string {
	if explicit != "" {
		return explicit
	}

	return t.accountID
}

func (t *client) do(ctx context.Context, method string, label string, query url.Values, in proto.Message, out proto.Message) error {
	var body io.Reader
	if in != nil {
		encoded, err := protojson.Marshal(in)
		if err != nil {
			return errorsx.Wrap(err, "unable to encode request")
		}
		body = bytes.NewReader(encoded)
	}

	target := t.endpoint + "/"
	if label != "" {
		target = t.endpoint + "/" + url.PathEscape(label)
	}

	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return errorsx.Wrap(err, "unable to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		return errorsx.Wrap(err, "request failed")
	}
	defer httpx.AutoClose(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		return errorsx.Errorf("%s", resp.Status)
	}

	if out == nil {
		return nil
	}

	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorsx.Wrap(err, "unable to read response")
	}

	return errorsx.Wrap(protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(encoded, out), "unable to decode response")
}
