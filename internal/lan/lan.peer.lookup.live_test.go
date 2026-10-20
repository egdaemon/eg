package lan_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/stretchr/testify/require"
)

func TestPeerLookupLive(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, lan.Migrate(t.Context(), db))

	for _, id := range []string{"fresh", "stale", "pinned", "short", "disabled"} {
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(id))).Scan(&lan.Peer{}))
	}

	var p lan.Peer
	require.NoError(t, lan.PeerFindByID(t.Context(), db, "pinned").Scan(&p))
	p.ExpiresAfter = -1 // never expires.
	require.NoError(t, lan.PeerUpdate(t.Context(), db, p).Scan(&p))

	require.NoError(t, lan.PeerFindByID(t.Context(), db, "short").Scan(&p))
	p.ExpiresAfter = int64(time.Hour)
	require.NoError(t, lan.PeerUpdate(t.Context(), db, p).Scan(&p))

	sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '31 days' WHERE peer_id = 'stale'`)
	sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '365 days' WHERE peer_id = 'pinned'`)
	sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '2 hours' WHERE peer_id = 'short'`)

	t.Run("honors per peer overrides", func(t *testing.T) {
		var peers []lan.Peer
		require.NoError(t, sqlx.ScanInto(lan.PeerLookupLive(t.Context(), db, int64(lan.DefaultTTL)), &peers))
		require.ElementsMatch(t, []string{"fresh", "pinned", "disabled"}, slicesx.MapTransform(func(p lan.Peer) string { return p.PeerID }, peers...))
	})

	t.Run("default ttl is configurable", func(t *testing.T) {
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '2 minutes' WHERE peer_id = 'fresh'`)

		var peers []lan.Peer
		require.NoError(t, sqlx.ScanInto(lan.PeerLookupLive(t.Context(), db, int64(time.Minute)), &peers))
		require.ElementsMatch(t, []string{"pinned", "disabled"}, slicesx.MapTransform(func(p lan.Peer) string { return p.PeerID }, peers...))
	})
}
