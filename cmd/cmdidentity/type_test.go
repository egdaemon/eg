package cmdidentity_test

import (
	"context"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestResolve(t *testing.T) {
	const vcsuri = "git@github.com:u/r.git"

	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
	require.NoError(t, err)

	t.Run("explicit account wins without any lookup", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		ident := cmdidentity.New(context.Background(), "explicit", nil)

		// a nil client would panic if the network were used.
		r, err := ident.Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, "explicit", r.Account)
		require.Equal(t, identity.SourceFlag, r.Source)
	})

	t.Run("explicit account is not overridden by the remembered account", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		remember(t, signer, vcsuri, "remembered")

		ident := cmdidentity.New(context.Background(), "explicit", nil)
		r, err := ident.Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, "explicit", r.Account)
	})

	t.Run("blank vcs uri is rejected", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		ident := cmdidentity.New(context.Background(), "", nil)

		_, err := ident.Resolve(nil, signer, "", false)
		require.ErrorContains(t, err, "a repository is required")
	})

	t.Run("remembered account is used without a lookup", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		remember(t, signer, vcsuri, "remembered")

		ident := cmdidentity.New(context.Background(), "", nil)
		r, err := ident.Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, "remembered", r.Account)
		require.Equal(t, identity.SourceCache, r.Source)
	})

	t.Run("result is memoized", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		remember(t, signer, vcsuri, "first")

		ident := cmdidentity.New(context.Background(), "", nil)
		r, err := ident.Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, "first", r.Account)

		remember(t, signer, vcsuri, "second")
		r, err = ident.Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, "first", r.Account)
	})
}

func remember(t *testing.T, signer ssh.Signer, vcsuri, account string) {
	t.Helper()

	db, err := migrations.OpenLocked(context.Background(), migrations.Path(migrations.CLI))
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, identity.Remember(context.Background(), db, signer, vcsuri, account))
}

func TestAccountID(t *testing.T) {
	account, err := cmdidentity.AccountID(&identity.Resolution{Account: "account"}, nil)
	require.NoError(t, err)
	require.Equal(t, "account", account)

	// a failed resolution can still carry candidates, the account must not leak out.
	account, err = cmdidentity.AccountID(&identity.Resolution{Account: "account"}, identity.ErrAmbiguous{})
	require.Error(t, err)
	require.Equal(t, "", account)
}

func TestChoose(t *testing.T) {
	profiles := []*authn.Authn{
		{Token: "token-1", Profile: &authn.Profile{Id: "profile-1"}},
		{Token: "token-2", Profile: &authn.Profile{Id: "profile-2"}},
	}

	t.Run("delegates to the login the identity was built with", func(t *testing.T) {
		var offered []*authn.Authn
		ident := cmdidentity.New(context.Background(), "", func(p []*authn.Authn) (*authn.Authn, error) {
			offered = p
			return p[1], nil
		})

		chosen, err := ident.Choose(profiles)
		require.NoError(t, err)
		require.Same(t, profiles[1], chosen)
		require.Equal(t, profiles, offered)
	})

	t.Run("returns the login's failure", func(t *testing.T) {
		ident := cmdidentity.New(context.Background(), "", func([]*authn.Authn) (*authn.Authn, error) {
			return nil, cmdidentity.ErrIdentityAmbiguous
		})

		chosen, err := ident.Choose(profiles)
		require.ErrorIs(t, err, cmdidentity.ErrIdentityAmbiguous)
		require.Nil(t, chosen)
	})
}

func TestStore(t *testing.T) {
	t.Run("opens the shared database", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		ident := cmdidentity.New(context.Background(), "", nil)

		db, err := ident.Store()
		require.NoError(t, err)
		defer db.Close()
		require.NoError(t, db.PingContext(context.Background()))
	})

	t.Run("remembered accounts are shared between stores", func(t *testing.T) {
		t.Setenv(userx.EnvCacheDirectory, t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)
		ident := cmdidentity.New(context.Background(), "", nil)

		db, err := ident.Store()
		require.NoError(t, err)
		require.NoError(t, identity.Remember(context.Background(), db, signer, "git@github.com:u/r.git", "remembered"))
		require.NoError(t, db.Close())

		db, err = ident.Store()
		require.NoError(t, err)
		defer db.Close()
		account, err := identity.Remembered(context.Background(), db, signer, "git@github.com:u/r.git")
		require.NoError(t, err)
		require.Equal(t, "remembered", account)
	})
}
