package lan_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	t.Run("remembers what the scan finds", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, lan.Migrate(t.Context(), db))

		priv, _, err := crypto.GenerateEd25519Key(nil)
		require.NoError(t, err)
		found, err := peer.IDFromPrivateKey(priv)
		require.NoError(t, err)

		var waited time.Duration
		require.NoError(t, lan.Discover(t.Context(), db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			waited = timeout
			return iterx.Slice([]peer.AddrInfo{{ID: found, Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/10.0.0.9/tcp/4001")}}}...)
		}, 7*time.Second))

		require.Equal(t, 7*time.Second, waited, "the scan is given the timeout")
		require.NoError(t, lan.PeerFindByID(t.Context(), db, found.String()).Scan(&lan.Peer{}))
	})

	t.Run("scan failures are reported and nothing is remembered", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, lan.Migrate(t.Context(), db))

		require.ErrorContains(t, lan.Discover(t.Context(), db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			return iterx.Error[peer.AddrInfo](errors.New("no multicast"))
		}, time.Second), "no multicast")

		require.Zero(t, sqltestx.Count(t, db, `SELECT count(*) FROM lan_peers`))
	})
}
