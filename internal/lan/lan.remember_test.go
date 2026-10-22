package lan_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestRemember(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	known, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)

	priv, _, err = crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	fresh, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)

	addr := multiaddr.StringCast("/ip4/10.0.0.7/tcp/4001")

	t.Run("remembers new systems", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, lan.Remember(t.Context(), db, peer.AddrInfo{ID: fresh, Addrs: []multiaddr.Multiaddr{addr}}))

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, fresh.String()).Scan(&p))
		require.Equal(t, "discovered via mdns", p.Description)
		require.Equal(t, []string{"/ip4/10.0.0.7/tcp/4001"}, p.AddrList())
		require.NotEmpty(t, p.PublicKey)
	})

	t.Run("keeps what is known about existing systems and refreshes their addresses", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(known.String()))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(known.String()), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))

		require.NoError(t, lan.Remember(t.Context(), db, peer.AddrInfo{ID: known, Addrs: []multiaddr.Multiaddr{addr}}))

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, known.String()).Scan(&p))
		require.Equal(t, []string{"/ip4/10.0.0.7/tcp/4001"}, p.AddrList())
		require.Equal(t, int64(8), p.Cores, "resources are kept")
		require.True(t, p.Disabled, "operator settings are kept")
	})
}
