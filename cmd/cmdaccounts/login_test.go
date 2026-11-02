package cmdaccounts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/httptestx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func nopicker(t *testing.T) func([]*authn.Authn) (*authn.Authn, error) {
	t.Helper()
	return func(_ []*authn.Authn) (*authn.Authn, error) {
		t.Fatal("unexpected picker invocation")
		return nil, nil
	}
}

func TestLoginRun(t *testing.T) {
	t.Run("no profiles associated with the key returns a notification", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				require.False(t, req.URL.Query().Has("account_id"))
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"token":"stub-options-token"}`))), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "", oauthc, oauthc, signer, nopicker(t))
		require.Error(t, err)
		require.ErrorContains(t, err, "not associated with any profiles")
	})

	t.Run("single profile establishes a session", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				require.False(t, req.URL.Query().Has("account_id"))
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"token":"stub-options-token"}`))), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{
				Profiles: []*authn.Authn{
					{Token: "the-token", Profile: &authn.Profile{Id: "profile-1", Display: "dave"}},
				},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, "/authn/current", req.URL.Path)
			require.Equal(t, "BEARER the-token", req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "account-1", Description: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "", oauthc, chttp, signer, nopicker(t))
		require.NoError(t, err)
	})

	t.Run("account flag fetches and forwards a signed login options token", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/authn/s/":
				require.Equal(t, "account-1", req.URL.Query().Get("account_id"))
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"token":"stub-options-token"}`))), Header: http.Header{}}
			case req.Method == http.MethodPost && req.URL.Path == "/authn/ssh":
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)

				var decoded struct {
					Token string `json:"token"`
				}
				require.NoError(t, json.Unmarshal(body, &decoded))
				require.Equal(t, "stub-options-token", decoded.Token)

				encoded, err := json.Marshal(authn.Authed{
					Profiles: []*authn.Authn{
						{Token: "the-token", Profile: &authn.Profile{Id: "profile-1", Display: "dave"}},
					},
				})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			}
		})

		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "account-1", Description: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "account-1", oauthc, chttp, signer, nopicker(t))
		require.NoError(t, err)
	})

	t.Run("multiple profiles defers to the picker", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			encoded, err := json.Marshal(authn.Authed{
				Profiles: []*authn.Authn{
					{Token: "token-1", Profile: &authn.Profile{Id: "profile-1", AccountId: "account-1"}},
					{Token: "token-2", Profile: &authn.Profile{Id: "profile-2", AccountId: "account-2"}},
				},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, "BEARER token-2", req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-2", Display: "dave"},
				Account: &authn.Account{Id: "account-2", Description: "dave's other account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		picker := func(profiles []*authn.Authn) (*authn.Authn, error) {
			require.Len(t, profiles, 2)
			require.Equal(t, "account-1", profiles[0].Profile.AccountId)
			require.Equal(t, "account-2", profiles[1].Profile.AccountId)
			return profiles[1], nil
		}

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "", oauthc, chttp, signer, picker)
		require.NoError(t, err)
	})

	t.Run("picker failure is propagated", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			encoded, err := json.Marshal(authn.Authed{
				Profiles: []*authn.Authn{
					{Token: "token-1", Profile: &authn.Profile{Id: "profile-1"}},
					{Token: "token-2", Profile: &authn.Profile{Id: "profile-2"}},
				},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		picker := func(profiles []*authn.Authn) (*authn.Authn, error) {
			return nil, errors.New("picker failed")
		}

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "", oauthc, oauthc, signer, picker)
		require.Error(t, err)
	})

	t.Run("loginssh failure is propagated", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "", oauthc, oauthc, signer, nopicker(t))
		require.Error(t, err)
	})

	t.Run("session failure is propagated", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			encoded, err := json.Marshal(authn.Authed{
				Profiles: []*authn.Authn{
					{Token: "the-token", Profile: &authn.Profile{Id: "profile-1"}},
				},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), "", oauthc, chttp, signer, nopicker(t))
		require.Error(t, err)
	})

	t.Run("account flag that is an account id is sent as the account id", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"account_id": "da832aa9-fb97-434e-98c2-008130552c9d"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				require.Equal(t, "da832aa9-fb97-434e-98c2-008130552c9d", req.URL.Query().Get("account_id"))
				require.True(t, strings.HasPrefix(req.URL.Host, "sso."), req.URL.Host)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			encoded, err := json.Marshal(authn.Authed{})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key", Account: "da832aa9-fb97-434e-98c2-008130552c9d"}.run(context.Background(), "da832aa9-fb97-434e-98c2-008130552c9d", oauthc, oauthc, signer, nopicker(t))
		require.ErrorContains(t, err, "not associated with any profiles")
	})

	t.Run("account flag that is the nil uuid is rejected without a request", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil
		})

		nilid := "00000000-0000-0000-0000-000000000000"
		err = Login{SSHKeyPath: "/path/to/key", Account: nilid}.run(context.Background(), nilid, oauthc, oauthc, signer, nopicker(t))
		require.ErrorContains(t, err, "account must be the account id or a eg login domain")
	})

	t.Run("account flag that is a login domain is sent as the subdomain", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": "domain-1", "account_id": "da832aa9-fb97-434e-98c2-008130552c9d"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				require.True(t, strings.HasPrefix(req.URL.Host, "demo.sso."), req.URL.Host)
				require.Equal(t, "11111111-1111-1111-1111-111111111111", req.URL.Query().Get("account_id"), "the global account is always sent")
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			encoded, err := json.Marshal(authn.Authed{})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key", Account: "demo"}.run(context.Background(), "11111111-1111-1111-1111-111111111111", oauthc, oauthc, signer, nopicker(t))
		require.ErrorContains(t, err, "not associated with any profiles")
	})

	t.Run("account flag that is an unknown login domain fails", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, "/authn/s/", req.URL.Path, "must not continue to the ssh exchange")
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key", Account: "typo"}.run(context.Background(), "11111111-1111-1111-1111-111111111111", oauthc, oauthc, signer, nopicker(t))
		require.ErrorContains(t, err, "no account found for 'typo'")
	})
}
