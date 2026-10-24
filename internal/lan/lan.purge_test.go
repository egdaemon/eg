package lan_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestPurge(t *testing.T) {
	t.Run("purges expired peers", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("keep"))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("drop"))).Scan(&lan.Peer{}))
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '31 days' WHERE peer_id = 'drop'`)

		require.NoError(t, lan.Purge(t.Context(), db, lan.DefaultTTL))

		require.NoError(t, lan.PeerFindByID(t.Context(), db, "keep").Scan(&lan.Peer{}))
		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "drop").Scan(&lan.Peer{}), sql.ErrNoRows)
	})
}
