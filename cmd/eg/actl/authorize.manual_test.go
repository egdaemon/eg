package actl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestAuthorizeManualParsing(t *testing.T) {
	t.Run("missing id argument fails to parse", func(t *testing.T) {
		genparser := cmdtestx.Genparser(AuthorizeManual{}, kong.Vars{"vars_ssh_key_path": "/expected/path"})

		_, err := genparser(t).Parse([]string{"command"})
		require.Error(t, err)
	})

	t.Run("sshkeypath default resolves at parse time", func(t *testing.T) {
		genparser := cmdtestx.Genparser(AuthorizeManual{}, kong.Vars{"vars_ssh_key_path": "/expected/path"})

		_, err := genparser(t).Parse([]string{"command", "the-id"})
		require.NoError(t, err)
	})
}

func TestAuthorizeManualRun(t *testing.T) {
	newsigner := func(t *testing.T) ssh.Signer {
		t.Helper()
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)
		return signer
	}

	t.Run("single profile grants using the provided id", func(t *testing.T) {
		signer := newsigner(t)

		var granted registration.RegistrationGrantRequest
		c := newauthzclient(t, authzServerOptions{
			Authed: func() authn.Authed {
				return authn.Authed{Profiles: []*authn.Authn{
					{Token: "the-token", Profile: &authn.Profile{Id: "profile-1"}},
				}}
			},
			Grant: func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &granted))
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{}`))
			},
		})

		err := AuthorizeManual{ID: "the-id", Shared: true}.run(context.Background(), c, "", signer)
		require.NoError(t, err)
		require.Equal(t, "the-id", granted.Registration.Id)
		require.True(t, granted.Global)
	})

	t.Run("no profiles associated with the key fails with an actionable message", func(t *testing.T) {
		signer := newsigner(t)

		c := newauthzclient(t, authzServerOptions{
			Authed: func() authn.Authed {
				return authn.Authed{Identity: &authn.Identity{Id: "identity-1"}}
			},
		})

		err := AuthorizeManual{ID: "the-id"}.run(context.Background(), c, "", signer)
		require.Error(t, err)
		require.ErrorContains(t, err, "not associated with any profiles")
	})

	t.Run("multiple profiles fails asking for --account, matching login's wording", func(t *testing.T) {
		signer := newsigner(t)

		c := newauthzclient(t, authzServerOptions{
			Authed: func() authn.Authed {
				return authn.Authed{
					Identity: &authn.Identity{Id: "identity-1"},
					Profiles: []*authn.Authn{
						{Token: "token-1", Profile: &authn.Profile{Id: "profile-1"}},
						{Token: "token-2", Profile: &authn.Profile{Id: "profile-2"}},
					},
				}
			},
		})

		err := AuthorizeManual{ID: "the-id"}.run(context.Background(), c, "", signer)
		require.Error(t, err)
		require.ErrorContains(t, err, "specify one with --account")
	})

	t.Run("non-empty account fetches and forwards a signed login options token", func(t *testing.T) {
		signer := newsigner(t)

		var loginOptionsCalled bool
		c := newauthzclient(t, authzServerOptions{
			LoginOptions: func(w http.ResponseWriter, r *http.Request) {
				loginOptionsCalled = true
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `"stub-options-token"`)
			},
			Authed: func() authn.Authed {
				return authn.Authed{Profiles: []*authn.Authn{
					{Token: "the-token", Profile: &authn.Profile{Id: "profile-1"}},
				}}
			},
			Grant: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{}`))
			},
		})

		err := AuthorizeManual{ID: "the-id"}.run(context.Background(), c, "account-1", signer)
		require.NoError(t, err)
		require.True(t, loginOptionsCalled)
	})

	t.Run("grant failure is propagated", func(t *testing.T) {
		signer := newsigner(t)

		c := newauthzclient(t, authzServerOptions{
			Authed: func() authn.Authed {
				return authn.Authed{Profiles: []*authn.Authn{
					{Token: "the-token", Profile: &authn.Profile{Id: "profile-1"}},
				}}
			},
			Grant: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		})

		err := AuthorizeManual{ID: "the-id"}.run(context.Background(), c, "", signer)
		require.Error(t, err)
	})
}
