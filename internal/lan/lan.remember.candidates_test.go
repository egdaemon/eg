package lan_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestRememberCandidates(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	id, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)

	t.Run("records what the control plane reports", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, lan.RememberCandidates(t.Context(), db,
			&compute.Compute{P2Pid: id.String(), Description: "build box", Os: "linux", Arch: "arm64", Cores: 16, Memory: 32 << 30, Vram: 8 << 30, Labels: []string{"gpu"}, Uploadable: true},
			&compute.Compute{Description: "no peer id, ignored"},
		))

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, id.String()).Scan(&p))
		require.Equal(t, "build box", p.Description)
		require.Equal(t, "linux", p.OS)
		require.Equal(t, "arm64", p.Arch)
		require.Equal(t, int64(16), p.Cores)
		require.Equal(t, int64(32<<30), p.Memory)
		require.Equal(t, int64(8<<30), p.Vram)
		require.Equal(t, []string{"gpu"}, p.LabelList())
		require.True(t, p.Uploadable)
		require.NotEmpty(t, p.PublicKey)

		require.Equal(t, 1, sqltestx.Count(t, db, `SELECT count(*) FROM lan_peers`), "candidates without a peer id are skipped")
	})

	t.Run("keeps the addresses and operator settings already known", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, lan.PeerUpsert(t.Context(), db, lan.Peer{PeerID: id.String(), Addrs: lan.EncodeList([]string{"/ip4/10.0.0.3/tcp/4001"}), Disk: 500}).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, lan.Peer{PeerID: id.String(), Description: "mine", Disabled: true, ExpiresAfter: -1}).Scan(&lan.Peer{}))

		require.NoError(t, lan.RememberCandidates(t.Context(), db, &compute.Compute{P2Pid: id.String(), Description: "reported", Cores: 4}))

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, id.String()).Scan(&p))
		require.Equal(t, []string{"/ip4/10.0.0.3/tcp/4001"}, p.AddrList())
		require.True(t, p.Disabled)
		require.Equal(t, int64(-1), p.ExpiresAfter)
		require.Equal(t, int64(500), p.Disk)
		require.Equal(t, int64(4), p.Cores)
		require.Equal(t, "reported", p.Description)
	})
}
