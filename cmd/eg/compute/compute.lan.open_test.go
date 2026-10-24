package compute

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestOpenLAN(t *testing.T) {
	t.Run("the daemon's database is reached through the daemon while it runs", func(t *testing.T) {
		// unix socket paths are length limited, keep the directory short.
		runtime, err := os.MkdirTemp("", "eg")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(runtime) })
		t.Setenv("RUNTIME_DIRECTORY", runtime)

		// the daemon holds the file open and serves it.
		daemon := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer daemon.Close()
		require.NoError(t, lan.PeerUpsert(t.Context(), daemon, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("keep"))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpsert(t.Context(), daemon, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("drop"))).Scan(&lan.Peer{}))
		sqltestx.Exec(t, daemon, `UPDATE lan_peers SET last_seen = now() - INTERVAL '31 days' WHERE peer_id = 'drop'`)

		ctx, done := context.WithCancel(t.Context())
		defer done()
		go func() {
			_ = duckproxyserver.ListenUnix(ctx, filepath.Join(runtime, migrations.DatabaseSocketName), duckproxyserver.New(daemon))
		}()

		require.Eventually(t, func() bool {
			_, err := os.Stat(filepath.Join(runtime, migrations.DatabaseSocketName))
			return err == nil
		}, 5*time.Second, 10*time.Millisecond)

		// the cache directory holds no database, the file is not opened.
		db, err := openlan(t.Context(), filepath.Join(t.TempDir(), migrations.Daemon), lan.DefaultTTL)
		require.NoError(t, err)
		defer db.Close()

		require.NoError(t, lan.PeerFindByID(t.Context(), db, "keep").Scan(&lan.Peer{}))
		require.Error(t, lan.PeerFindByID(t.Context(), db, "drop").Scan(&lan.Peer{}), "expired peers are purged through the proxy")
	})

	t.Run("opens the file when the daemon is not running", func(t *testing.T) {
		t.Setenv("RUNTIME_DIRECTORY", t.TempDir())

		dir := t.TempDir()
		db, err := openlan(t.Context(), filepath.Join(dir, migrations.Daemon), lan.DefaultTTL)
		require.NoError(t, err)
		require.NoError(t, db.Close())

		_, err = os.Stat(filepath.Join(dir, migrations.Daemon))
		require.NoError(t, err)
	})

	t.Run("defaults to the client's database", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CACHE_DIRECTORY", dir)

		db, err := openlan(t.Context(), "", lan.DefaultTTL)
		require.NoError(t, err)
		require.NoError(t, db.Close())

		_, err = os.Stat(filepath.Join(dir, migrations.CLI))
		require.NoError(t, err)
	})
}
