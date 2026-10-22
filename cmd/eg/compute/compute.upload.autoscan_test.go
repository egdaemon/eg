package compute

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestAutoscan(t *testing.T) {
	t.Run("doesn't block the caller and records what is found", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		priv, _, err := crypto.GenerateEd25519Key(nil)
		require.NoError(t, err)
		found, err := peer.IDFromPrivateKey(priv)
		require.NoError(t, err)

		release := make(chan struct{})
		started := time.Now()
		stop := autoscan(t.Context(), db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			<-release
			return iterx.Slice([]peer.AddrInfo{{ID: found, Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/10.0.0.9/tcp/4001")}}}...)
		}, time.Second)

		require.Less(t, time.Since(started), time.Second, "the scan must not hold up the caller")
		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, found.String()).Scan(&lan.Peer{}), sql.ErrNoRows, "nothing is known until the scan finishes")

		close(release)
		require.Eventually(t, func() bool {
			return sqltestx.Count(t, db, `SELECT count(*) FROM lan_peers`) == 1
		}, 5*time.Second, 10*time.Millisecond)
		stop()

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, found.String()).Scan(&p))
		require.Equal(t, []string{"/ip4/10.0.0.9/tcp/4001"}, p.AddrList())
	})

	t.Run("failures are ignored", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		stop := autoscan(t.Context(), db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			return iterx.Error[peer.AddrInfo](errors.New("no multicast"))
		}, time.Second)

		require.NotPanics(t, stop)
	})

	t.Run("stopping cancels a scan in flight", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		stop := autoscan(t.Context(), db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			<-ctx.Done()
			return iterx.Error[peer.AddrInfo](ctx.Err())
		}, time.Minute)

		started := time.Now()
		stop()
		require.Less(t, time.Since(started), 5*time.Second)
	})
}
