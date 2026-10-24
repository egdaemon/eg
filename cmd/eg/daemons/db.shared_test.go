package daemons_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/history"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

// the gate and the history store share the daemon's single database handle.
func TestDaemonDatabaseShared(t *testing.T) {
	db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
	defer db.Close()

	gate := daemons.NewLANGate(db, lan.DefaultTTL)
	addr := multiaddr.StringCast("/ip4/192.168.1.20/tcp/4001")

	const n = 8
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			priv, _, err := crypto.GenerateEd25519Key(nil)
			require.NoError(t, err)
			remote, err := peer.IDFromPrivateKey(priv)
			require.NoError(t, err)
			require.True(t, gate.Allow(t.Context(), remote, addr))
		}()
		go func() {
			defer wg.Done()
			require.NoError(t, history.EntryUpsert(t.Context(), db, langx.Clone(history.Entry{}, history.EntryOptionTestDefaults, history.EntryOptionID(fmt.Sprintf("w%d", i)))).Scan(&history.Entry{}))
		}()
	}
	wg.Wait()

	for i := range n {
		require.NoError(t, history.EntryFindByID(t.Context(), db, fmt.Sprintf("w%d", i)).Scan(&history.Entry{}))
	}

	var peers []lan.Peer
	require.NoError(t, sqlx.ScanInto(lan.PeerLookupAll(t.Context(), db), &peers))
	require.Len(t, peers, n)
}
