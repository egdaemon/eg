package lan_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestPeerTouch(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, migrations.Migrate(t.Context(), db))
	require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))
	sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '29 days' WHERE peer_id = 'a'`)

	var p lan.Peer
	require.NoError(t, lan.PeerTouch(t.Context(), db, "a").Scan(&p))
	require.WithinDuration(t, time.Now(), p.LastSeen, time.Minute)
}
