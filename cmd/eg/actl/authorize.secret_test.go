package actl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeSecretSigner(t *testing.T) {
	t.Run("same seed derives the same identity", func(t *testing.T) {
		s1, err := AuthorizeSecret{Seed: "same-seed"}.Signer(cmdopts.GenerateEntropy, sshx.NewKeyGenSeeded)
		require.NoError(t, err)

		s2, err := AuthorizeSecret{Seed: "same-seed"}.Signer(cmdopts.GenerateEntropy, sshx.NewKeyGenSeeded)
		require.NoError(t, err)

		require.Equal(t, s1.PublicKey().Marshal(), s2.PublicKey().Marshal())
	})

	t.Run("blank seed is rejected", func(t *testing.T) {
		_, err := AuthorizeSecret{}.Signer(cmdopts.GenerateEntropy, sshx.NewKeyGenSeeded)
		require.ErrorContains(t, err, "seed is required")
	})

	t.Run("different seeds derive different identities", func(t *testing.T) {
		s1, err := AuthorizeSecret{Seed: "seed-one"}.Signer(cmdopts.GenerateEntropy, sshx.NewKeyGenSeeded)
		require.NoError(t, err)

		s2, err := AuthorizeSecret{Seed: "seed-two"}.Signer(cmdopts.GenerateEntropy, sshx.NewKeyGenSeeded)
		require.NoError(t, err)

		require.NotEqual(t, s1.PublicKey().Marshal(), s2.PublicKey().Marshal())
	})
}

func TestAuthorizeSecretParsing(t *testing.T) {
	t.Run("missing seed argument fails to parse", func(t *testing.T) {
		genparser := cmdtestx.Genparser(AuthorizeSecret{}, kong.Vars{"vars_ssh_key_path": "/expected/path", "env_eg_entropy_seed": eg.EnvEGEntropySeed, "vars_ssh_key_seed": "default-ssh-seed", "env_eg_ssh_key_seed": eg.EnvEGSSHSeed})

		_, err := genparser(t).Parse([]string{"command"})
		require.Error(t, err)
	})

	t.Run("seed falls back to the environment", func(t *testing.T) {
		t.Setenv(eg.EnvEGEntropySeed, "env-seed")
		genparser := cmdtestx.Genparser(AuthorizeSecret{}, kong.Vars{"vars_ssh_key_path": "/expected/path", "env_eg_entropy_seed": eg.EnvEGEntropySeed, "vars_ssh_key_seed": "default-ssh-seed", "env_eg_ssh_key_seed": eg.EnvEGSSHSeed})

		kctx, err := genparser(t).Parse([]string{"command"})
		require.NoError(t, err)
		require.Equal(t, "env-seed", kctx.Selected().Positional[0].Target.String())
	})

	t.Run("explicit seed wins over the environment", func(t *testing.T) {
		t.Setenv(eg.EnvEGEntropySeed, "env-seed")
		genparser := cmdtestx.Genparser(AuthorizeSecret{}, kong.Vars{"vars_ssh_key_path": "/expected/path", "env_eg_entropy_seed": eg.EnvEGEntropySeed, "vars_ssh_key_seed": "default-ssh-seed", "env_eg_ssh_key_seed": eg.EnvEGSSHSeed})

		kctx, err := genparser(t).Parse([]string{"command", "arg-seed"})
		require.NoError(t, err)
		require.Equal(t, "arg-seed", kctx.Selected().Positional[0].Target.String())
	})

	t.Run("ssh seed falls back to the ssh key seed environment", func(t *testing.T) {
		t.Setenv(eg.EnvEGSSHSeed, "env-ssh-seed")
		genparser := cmdtestx.Genparser(AuthorizeSecret{}, kong.Vars{"vars_ssh_key_path": "/expected/path", "env_eg_entropy_seed": eg.EnvEGEntropySeed, "vars_ssh_key_seed": "default-ssh-seed", "env_eg_ssh_key_seed": eg.EnvEGSSHSeed})

		kctx, err := genparser(t).Parse([]string{"command", "arg-seed"})
		require.NoError(t, err)

		var found bool
		for _, flag := range kctx.Selected().Flags {
			if flag.Name == "sshkeyseed" {
				found = true
				require.Equal(t, "env-ssh-seed", flag.Target.String())
			}
		}
		require.True(t, found)
	})

	t.Run("sshkeypath default resolves at parse time", func(t *testing.T) {
		genparser := cmdtestx.Genparser(AuthorizeSecret{}, kong.Vars{"vars_ssh_key_path": "/expected/path", "env_eg_entropy_seed": eg.EnvEGEntropySeed, "vars_ssh_key_seed": "default-ssh-seed", "env_eg_ssh_key_seed": eg.EnvEGSSHSeed})

		_, err := genparser(t).Parse([]string{"command", "00000000-0000-0000-0000-000000000000"})
		require.NoError(t, err)
	})
}

func TestAuthorizeSecretRun(t *testing.T) {
	t.Run("single profile grants using the derived regid", func(t *testing.T) {
		derived, err := cmdopts.GenerateEntropy("the-seed")
		require.NoError(t, err)

		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(derived))
		require.NoError(t, err)

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

		err = AuthorizeSecret{Shared: true}.run(context.Background(), c, "", "", signer.PublicKey(), signer)
		require.NoError(t, err)
		require.Equal(t, identity.ID(signer.PublicKey()), granted.Registration.Id)
		require.True(t, granted.Global)
	})

	t.Run("no profiles associated with the key fails with an actionable message", func(t *testing.T) {
		derived, err := cmdopts.GenerateEntropy("the-seed")
		require.NoError(t, err)

		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(derived))
		require.NoError(t, err)

		c := newauthzclient(t, authzServerOptions{
			Authed: func() authn.Authed {
				return authn.Authed{Identity: &authn.Identity{Id: "identity-1"}}
			},
		})

		err = AuthorizeSecret{}.run(context.Background(), c, "", "", signer.PublicKey(), signer)
		require.Error(t, err)
		require.ErrorContains(t, err, "not associated with any profiles")
	})

	t.Run("multiple profiles fails asking for --account, matching login's wording", func(t *testing.T) {
		derived, err := cmdopts.GenerateEntropy("the-seed")
		require.NoError(t, err)

		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(derived))
		require.NoError(t, err)

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

		err = AuthorizeSecret{}.run(context.Background(), c, "", "", signer.PublicKey(), signer)
		require.Error(t, err)
		require.ErrorContains(t, err, "specify one with --account")
	})

	t.Run("non-empty account fetches and forwards a signed login options token", func(t *testing.T) {
		derived, err := cmdopts.GenerateEntropy("the-seed")
		require.NoError(t, err)

		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(derived))
		require.NoError(t, err)

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

		err = AuthorizeSecret{}.run(context.Background(), c, "account-1", "", signer.PublicKey(), signer)
		require.NoError(t, err)
		require.True(t, loginOptionsCalled)
	})

	t.Run("grant failure is propagated", func(t *testing.T) {
		derived, err := cmdopts.GenerateEntropy("the-seed")
		require.NoError(t, err)

		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(derived))
		require.NoError(t, err)

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

		err = AuthorizeSecret{}.run(context.Background(), c, "", "", signer.PublicKey(), signer)
		require.Error(t, err)
	})
}
