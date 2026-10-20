package lan_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/stretchr/testify/require"
)

func TestPeerLookupAll(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, lan.Migrate(t.Context(), db))
	require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))
	require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("b"))).Scan(&lan.Peer{}))
	sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '90 days' WHERE peer_id = 'b'`)

	var peers []lan.Peer
	require.NoError(t, sqlx.ScanInto(lan.PeerLookupAll(t.Context(), db), &peers))
	require.Equal(t, []string{"a", "b"}, slicesx.MapTransform(func(p lan.Peer) string { return p.PeerID }, peers...), "includes expired, most recently seen first")
}
