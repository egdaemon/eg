package lan_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestPeerDeleteByID(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, lan.Migrate(t.Context(), db))
	require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))

	var p lan.Peer
	require.NoError(t, lan.PeerDeleteByID(t.Context(), db, "a").Scan(&p))
	require.Equal(t, "a", p.PeerID)

	require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "a").Scan(&p), sql.ErrNoRows)
	require.ErrorIs(t, lan.PeerDeleteByID(t.Context(), db, "a").Scan(&p), sql.ErrNoRows, "deleting a missing peer reports no rows")
}
