package identity

import (
	"context"
	"database/sql"
	"github.com/egdaemon/eg/cmd/migrations"
	"path/filepath"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func testsigner(t *testing.T) ssh.Signer {
	t.Helper()
	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
	require.NoError(t, err)
	return signer
}

func profile(account string) *authn.Authn {
	return &authn.Authn{Token: "token-" + account, Account: &authn.Account{Id: account}, Profile: &authn.Profile{Id: "profile-" + account, AccountId: account}}
}

func lookup(profiles ...*authn.Authn) func() (*authn.Authed, error) {
	return func() (*authn.Authed, error) {
		return &authn.Authed{Profiles: profiles}, nil
	}
}

func TestHints(t *testing.T) {
	ctx := context.Background()
	signer := testsigner(t)
	const uri = "git@github.com:u/r.git"
	db, err := migrations.Open(ctx, filepath.Join(t.TempDir(), "identity.duckdb"))
	require.NoError(t, err)
	defer db.Close()

	got, err := remembered(ctx, db, signer, QueryVCSURI(uri))
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, Remember(ctx, db, signer, uri, &authn.Identity{}, profile("a1"), true))
	require.NoError(t, Remember(ctx, db, signer, uri, &authn.Identity{}, profile("a2"), true))
	got, err = remembered(ctx, db, signer, squirrel.And{QueryVCSURI(uri), QueryDefault()})
	require.NoError(t, err)
	require.Equal(t, "a2", got.AccountID)
	require.Equal(t, "profile-a2", got.ProfileID)

	// the previous identity is still remembered, it is no longer the default.
	got, err = remembered(ctx, db, signer, squirrel.And{QueryVCSURI(uri), QueryProfileID("profile-a1")})
	require.NoError(t, err)
	require.Equal(t, "a1", got.AccountID)
	require.False(t, got.Default)

	got, err = remembered(ctx, db, signer, squirrel.And{QueryVCSURI(uri), QueryAccountID("a3")})
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = remembered(ctx, db, signer, QueryVCSURI("git@github.com:u/other.git"))
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, Forget(ctx, db, signer, uri))
	got, err = remembered(ctx, db, signer, QueryVCSURI(uri))
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestResolve(t *testing.T) {
	ctx := context.Background()
	const vcsuri = "git@github.com:u/r.git"
	opendb := func(t *testing.T) (db *sql.DB) {
		d, err := migrations.Open(ctx, filepath.Join(t.TempDir(), "identity.duckdb"))
		require.NoError(t, err)
		t.Cleanup(func() { d.Close() })
		return d
	}

	t.Run("single profile", func(t *testing.T) {
		r, err := QueryVCSAuto("", "", vcsuri).resolve(ctx, opendb(t), testsigner(t), nil, lookup(profile("a1")))
		require.NoError(t, err)
		require.Equal(t, "a1", r.Current.Account.Id)
		require.Equal(t, SourceCache, r.Source)
	})

	t.Run("a looked up profile is remembered and answers the next resolution", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		r, err := QueryVCSAuto("", "", vcsuri).resolve(ctx, db, signer, nil, lookup(profile("a2")))
		require.NoError(t, err)
		require.Equal(t, "a2", r.Current.Account.Id)

		// the second resolution is answered by the cache without consulting the server.
		r, err = QueryVCSAuto("", "", vcsuri).resolve(ctx, db, signer, nil, func() (*authn.Authed, error) {
			t.Fatal("unexpected lookup")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, "a2", r.Current.Account.Id)
		require.Equal(t, SourceCache, r.Source)
	})

	t.Run("forgetting the repository detects again", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		require.NoError(t, Remember(ctx, db, signer, vcsuri, &authn.Identity{}, profile("stale"), true))
		require.NoError(t, Forget(ctx, db, signer, vcsuri))
		r, err := QueryVCSAuto("", "", vcsuri).resolve(ctx, db, signer, nil, lookup(profile("a1")))
		require.NoError(t, err)
		require.Equal(t, "a1", r.Current.Account.Id)
		got, err := remembered(ctx, db, signer, squirrel.And{QueryVCSURI(vcsuri), QueryDefault()})
		require.NoError(t, err)
		require.Equal(t, "a1", got.AccountID)
	})

	t.Run("multiple profiles without a chooser is ambiguous", func(t *testing.T) {
		r, err := QueryVCSAuto("", "", vcsuri).resolve(ctx, opendb(t), testsigner(t), nil, lookup(profile("a1"), profile("a2")))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAmbiguous)
		require.Len(t, r.Candidates, 2)
	})

	t.Run("the ambiguity says which identity and how many candidates", func(t *testing.T) {
		_, err := QueryVCSAuto("", "", vcsuri).resolve(ctx, opendb(t), testsigner(t), nil, lookup(profile("a1"), profile("a2")))
		require.ErrorIs(t, err, ErrAmbiguous)
		require.ErrorContains(t, err, "2 candidate profiles")
		require.ErrorContains(t, err, "--account-id")
	})

	t.Run("blank vcs uri is ambiguous", func(t *testing.T) {
		_, err := QueryVCSAuto("", "", "").resolve(ctx, opendb(t), testsigner(t), nil, lookup(profile("a1"), profile("a2")))
		require.ErrorIs(t, err, ErrAmbiguous)
	})

	t.Run("a remembered profile answers a profile hint without a lookup", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		require.NoError(t, Remember(ctx, db, signer, vcsuri, &authn.Identity{}, profile("a1"), true))
		require.NoError(t, Remember(ctx, db, signer, vcsuri, &authn.Identity{}, profile("a2"), true))

		r, err := QueryEnvironment("", "profile-a1", vcsuri).resolve(ctx, db, signer, nil, func() (*authn.Authed, error) {
			t.Fatal("unexpected lookup")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, "a1", r.Current.Account.Id)
		require.Equal(t, "profile-a1", r.Current.Profile.Id)
		require.Equal(t, SourceFlag, r.Source)
	})

	t.Run("a profile hint is looked up and remembered as the default for the vcs auto query", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		r, err := QueryVCSAuto("", "profile-a2", vcsuri).resolve(ctx, db, signer, nil, lookup(profile("a2")))
		require.NoError(t, err)
		require.Equal(t, "profile-a2", r.Current.Profile.Id)
		require.Equal(t, SourceCache, r.Source)

		got, err := remembered(ctx, db, signer, squirrel.And{QueryVCSURI(vcsuri), QueryDefault()})
		require.NoError(t, err)
		require.Equal(t, "profile-a2", got.ProfileID)
	})

	t.Run("a profile hint is remembered as not the default for the environment query", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		_, err := QueryEnvironment("", "profile-a2", vcsuri).resolve(ctx, db, signer, nil, lookup(profile("a2")))
		require.NoError(t, err)

		got, err := remembered(ctx, db, signer, QueryProfileID("profile-a2"))
		require.NoError(t, err)
		require.False(t, got.Default)

		got, err = remembered(ctx, db, signer, QueryDefault())
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("an explicit account that is not remembered is looked up", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		require.NoError(t, Remember(ctx, db, signer, vcsuri, &authn.Identity{}, profile("a1"), true))

		r, err := QueryEnvironment("a2", "", vcsuri).resolve(ctx, db, signer, nil, lookup(profile("a2")))
		require.NoError(t, err)
		require.Equal(t, "a2", r.Current.Account.Id)
		require.Equal(t, SourceFlag, r.Source)

		got, err := remembered(ctx, db, signer, QueryAccountID("a2"))
		require.NoError(t, err)
		require.Equal(t, "a2", got.AccountID)
	})

	t.Run("an explicit account that is remembered needs no lookup", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		require.NoError(t, Remember(ctx, db, signer, vcsuri, &authn.Identity{}, profile("a1"), true))

		r, err := QueryEnvironment("a1", "", vcsuri).resolve(ctx, db, signer, nil, func() (*authn.Authed, error) {
			t.Fatal("unexpected lookup")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, "a1", r.Current.Account.Id)
		require.Equal(t, SourceFlag, r.Source)
	})
}
