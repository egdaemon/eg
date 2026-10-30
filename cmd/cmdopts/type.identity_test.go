package cmdopts_test

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestGlobalIdentityIsMemoized(t *testing.T) {
	g := &cmdopts.Global{}
	require.Same(t, g.Identity(), g.Identity())
}

func TestIdentityAccount(t *testing.T) {
	const vcsuri = "git@github.com:u/r.git"

	signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
	require.NoError(t, err)

	t.Run("explicit account wins without any lookup", func(t *testing.T) {
		t.Setenv("CACHE_DIRECTORY", t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		g := &cmdopts.Global{Context: context.Background(), AccountID: "explicit"}

		// a nil client would panic if the network were used.
		account, err := g.Identity().Account(nil, signer, vcsuri)
		require.NoError(t, err)
		require.Equal(t, "explicit", account)

		r, err := g.Identity().Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, identity.SourceFlag, r.Source)
	})

	t.Run("explicit account is not overridden by the remembered account", func(t *testing.T) {
		t.Setenv("CACHE_DIRECTORY", t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		remember(t, signer, vcsuri, "remembered")

		g := &cmdopts.Global{Context: context.Background(), AccountID: "explicit"}
		account, err := g.Identity().Account(nil, signer, vcsuri)
		require.NoError(t, err)
		require.Equal(t, "explicit", account)
	})

	t.Run("blank vcs uri is rejected", func(t *testing.T) {
		t.Setenv("CACHE_DIRECTORY", t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		g := &cmdopts.Global{Context: context.Background()}

		_, err := g.Identity().Account(nil, signer, "")
		require.ErrorContains(t, err, "a repository is required")
	})

	t.Run("remembered account is used without a lookup", func(t *testing.T) {
		t.Setenv("CACHE_DIRECTORY", t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		remember(t, signer, vcsuri, "remembered")

		g := &cmdopts.Global{Context: context.Background()}
		r, err := g.Identity().Resolve(nil, signer, vcsuri, false)
		require.NoError(t, err)
		require.Equal(t, "remembered", r.Account)
		require.Equal(t, identity.SourceCache, r.Source)
	})

	t.Run("result is memoized", func(t *testing.T) {
		t.Setenv("CACHE_DIRECTORY", t.TempDir())
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
		remember(t, signer, vcsuri, "first")

		g := &cmdopts.Global{Context: context.Background()}
		account, err := g.Identity().Account(nil, signer, vcsuri)
		require.NoError(t, err)
		require.Equal(t, "first", account)

		remember(t, signer, vcsuri, "second")
		account, err = g.Identity().Account(nil, signer, vcsuri)
		require.NoError(t, err)
		require.Equal(t, "first", account)
	})
}

func TestWorkspaceVCSURI(t *testing.T) {
	t.Run("repository with an origin remote", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/egdaemon/eg.git"}})
		require.NoError(t, err)
		t.Chdir(dir)

		require.Equal(t, "git@github.com:egdaemon/eg.git", cmdopts.WorkspaceVCSURI())
	})

	t.Run("repository without a remote is blank and silent", func(t *testing.T) {
		dir := t.TempDir()
		_, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		t.Chdir(dir)

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		require.Equal(t, "", cmdopts.WorkspaceVCSURI())
		require.Empty(t, logged.String())
	})

	t.Run("directory outside of a repository is blank and silent", func(t *testing.T) {
		t.Chdir(t.TempDir())

		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		require.Equal(t, "", cmdopts.WorkspaceVCSURI())
		require.Empty(t, logged.String())
	})
}

func remember(t *testing.T, signer ssh.Signer, vcsuri, account string) {
	t.Helper()

	db, err := migrations.OpenLocked(context.Background(), migrations.Path(migrations.CLI))
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, identity.Remember(context.Background(), db, signer, vcsuri, account))
}
