package lan_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("purges expired peers on reopen", func(t *testing.T) {
		path := sqltestx.Path(t)

		db := testx.MustT(lan.Open(t.Context(), path, lan.DefaultTTL))(t)
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("keep"))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("drop"))).Scan(&lan.Peer{}))
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '31 days' WHERE peer_id = 'drop'`)
		require.NoError(t, db.Close())

		db = testx.MustT(lan.Open(t.Context(), path, lan.DefaultTTL))(t)
		defer db.Close()

		require.NoError(t, lan.PeerFindByID(t.Context(), db, "keep").Scan(&lan.Peer{}))
		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "drop").Scan(&lan.Peer{}), sql.ErrNoRows)
	})

	t.Run("creates the parent directory", func(t *testing.T) {
		db := testx.MustT(lan.Open(t.Context(), t.TempDir()+"/missing/dir/lan.duckdb", lan.DefaultTTL))(t)
		require.NoError(t, db.Close())
	})
}
