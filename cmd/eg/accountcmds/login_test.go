package accountcmds

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/httptestx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
)

func TestLoginRun(t *testing.T) {
	t.Run("no profiles associated with the key returns a notification", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), oauthc, oauthc, signer)
		require.Error(t, err)
		require.ErrorContains(t, err, "not associated with any profiles")
	})

	t.Run("single profile establishes a session", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
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
				Account: &authn.Account{Id: "account-1", Display: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), oauthc, chttp, signer)
		require.NoError(t, err)
	})

	t.Run("multiple profiles are not yet supported", func(t *testing.T) {
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

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), oauthc, oauthc, signer)
		require.Error(t, err)
		require.ErrorContains(t, err, "already registered an account")
	})

	t.Run("loginssh failure is propagated", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}
		})

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), oauthc, oauthc, signer)
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

		err = Login{SSHKeyPath: "/path/to/key"}.run(context.Background(), oauthc, chttp, signer)
		require.Error(t, err)
	})
}
