package identity

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/httptestx"
	"github.com/egdaemon/eg/internal/lan"
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
	return &authn.Authn{Token: "token-" + account, Profile: &authn.Profile{Id: "profile-" + account, AccountId: account}}
}

func lookup(profiles ...*authn.Authn) func() (*authn.Authed, error) {
	return func() (*authn.Authed, error) {
		return &authn.Authed{Profiles: profiles}, nil
	}
}

// reposhaving answers /repositories searches, only the listed accounts have the repository.
func reposhaving(t *testing.T, has ...string) *http.Client {
	return httptestx.NewTestClient(func(req *http.Request) *http.Response {
		require.Equal(t, "/repositories/", req.URL.Path)
		items := []json.RawMessage{}
		for _, a := range has {
			if req.Header.Get("Authorization") == "BEARER token-"+a {
				items = append(items, json.RawMessage(`{}`))
			}
		}
		encoded, err := json.Marshal(map[string]any{"items": items})
		require.NoError(t, err)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
	})
}

func TestHints(t *testing.T) {
	ctx := context.Background()
	signer := testsigner(t)
	db, err := lan.Open(ctx, filepath.Join(t.TempDir(), "identity.duckdb"), lan.DefaultTTL)
	require.NoError(t, err)
	defer db.Close()

	got, err := Remembered(ctx, db, signer, "git@github.com:u/r.git")
	require.NoError(t, err)
	require.Empty(t, got)

	require.NoError(t, Remember(ctx, db, signer, "git@github.com:u/r.git", "a1"))
	require.NoError(t, Remember(ctx, db, signer, "git@github.com:u/r.git", "a2"))
	got, err = Remembered(ctx, db, signer, "git@github.com:u/r.git")
	require.NoError(t, err)
	require.Equal(t, "a2", got)

	got, err = Remembered(ctx, db, signer, "git@github.com:u/other.git")
	require.NoError(t, err)
	require.Empty(t, got)

	require.NoError(t, Forget(ctx, db, signer, "git@github.com:u/r.git"))
	got, err = Remembered(ctx, db, signer, "git@github.com:u/r.git")
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestResolve(t *testing.T) {
	ctx := context.Background()
	const vcsuri = "git@github.com:u/r.git"
	opendb := func(t *testing.T) (db *sql.DB) {
		d, err := lan.Open(ctx, filepath.Join(t.TempDir(), "identity.duckdb"), lan.DefaultTTL)
		require.NoError(t, err)
		t.Cleanup(func() { d.Close() })
		return d
	}

	t.Run("single profile", func(t *testing.T) {
		r, err := resolve(ctx, opendb(t), nil, testsigner(t), vcsuri, nil, false, lookup(profile("a1")))
		require.NoError(t, err)
		require.Equal(t, "a1", r.Account)
		require.Equal(t, SourceSingle, r.Source)
	})

	t.Run("repository narrows to one account and is remembered", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		r, err := resolve(ctx, db, reposhaving(t, "a2"), signer, vcsuri, nil, false, lookup(profile("a1"), profile("a2")))
		require.NoError(t, err)
		require.Equal(t, "a2", r.Account)
		require.Equal(t, SourceRepository, r.Source)

		// the second resolution is answered by the cache without consulting the server.
		r, err = resolve(ctx, db, nil, signer, vcsuri, nil, false, func() (*authn.Authed, error) {
			t.Fatal("unexpected lookup")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, "a2", r.Account)
		require.Equal(t, SourceCache, r.Source)
	})

	t.Run("refresh ignores the cache", func(t *testing.T) {
		db := opendb(t)
		signer := testsigner(t)
		require.NoError(t, Remember(ctx, db, signer, vcsuri, "stale"))
		r, err := resolve(ctx, db, reposhaving(t, "a1"), signer, vcsuri, nil, true, lookup(profile("a1"), profile("a2")))
		require.NoError(t, err)
		require.Equal(t, "a1", r.Account)
		got, err := Remembered(ctx, db, signer, vcsuri)
		require.NoError(t, err)
		require.Equal(t, "a1", got)
	})

	t.Run("no repository match without a chooser is ambiguous", func(t *testing.T) {
		r, err := resolve(ctx, opendb(t), reposhaving(t), testsigner(t), vcsuri, nil, false, lookup(profile("a1"), profile("a2")))
		require.Error(t, err)
		require.True(t, errors.As(err, &ErrAmbiguous{}))
		require.Len(t, r.Candidates, 2)
	})

	t.Run("multiple repository matches are the candidates", func(t *testing.T) {
		r, err := resolve(ctx, opendb(t), reposhaving(t, "a1", "a3"), testsigner(t), vcsuri, nil, false, lookup(profile("a1"), profile("a2"), profile("a3")))
		require.True(t, errors.As(err, &ErrAmbiguous{}))
		require.Len(t, r.Candidates, 2)
	})

	t.Run("blank vcs uri is ambiguous", func(t *testing.T) {
		_, err := resolve(ctx, opendb(t), nil, testsigner(t), "", nil, false, lookup(profile("a1"), profile("a2")))
		require.True(t, errors.As(err, &ErrAmbiguous{}))
	})
}
