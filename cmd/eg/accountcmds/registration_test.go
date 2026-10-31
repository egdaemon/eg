package accountcmds

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/httptestx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestSignupRun(t *testing.T) {
	t.Run("account id with an existing profile skips selection and logs in", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"account_id": "account-1"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/authn/s/":
				require.Equal(t, "account-1", req.URL.Query().Get("account_id"))
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			case req.Method == http.MethodPost && req.URL.Path == "/authn/ssh":
				encoded, err := json.Marshal(authn.Authed{
					Profiles: []*authn.Authn{
						{Token: "the-token", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "account-1"}},
					},
				})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			}
		})

		var sessions int
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			// neither signup nor register may be requested when a profile already exists.
			require.Equal(t, "/authn/current", req.URL.Path)
			require.Equal(t, "BEARER the-token", req.Header.Get("Authorization"))
			sessions++
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "account-1", Display: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{AccountID: "account-1", Context: context.Background()}, oauthc, chttp, signer)
		require.NoError(t, err)
		require.Equal(t, 1, sessions)
	})
}
