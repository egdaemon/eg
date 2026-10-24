package compute

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestLanList(t *testing.T) {
	gctx := &cmdopts.Global{Context: t.Context()}

	t.Run("lists remembered systems", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		for _, id := range []string{"alpha", "beta", "gamma"} {
			require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(id))).Scan(&lan.Peer{}))
		}
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("beta"), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("gamma"), func(p *lan.Peer) {
			p.ExpiresAfter = -1
		})).Scan(&lan.Peer{}))
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '40 days' WHERE peer_id = 'alpha'`)

		var out bytes.Buffer
		require.NoError(t, lanList{PeerTTL: lan.DefaultTTL}.list(gctx, db, nil, &out))

		got := out.String()
		require.Contains(t, got, "ID")
		require.NotContains(t, got, "alpha", "expired systems are not listed")
		require.Contains(t, got, "beta")
		require.Contains(t, got, "disabled")
		require.Contains(t, got, "gamma")
		require.Contains(t, got, "never")
		require.Contains(t, got, "linux/amd64")
		require.Contains(t, got, "gpu,fast")
	})

	t.Run("scan remembers new systems and keeps what is known about existing ones", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		newid := func() peer.ID {
			priv, _, err := crypto.GenerateEd25519Key(nil)
			require.NoError(t, err)
			id, err := peer.IDFromPrivateKey(priv)
			require.NoError(t, err)
			return id
		}
		known, fresh := newid(), newid()
		addr := multiaddr.StringCast("/ip4/10.0.0.7/tcp/4001")

		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(known.String()))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(known.String()), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))

		var out bytes.Buffer
		cmd := lanList{
			PeerTTL:     lan.DefaultTTL,
			Scan:        true,
			ScanTimeout: time.Second,
		}
		require.NoError(t, cmd.list(gctx, db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			return iterx.Slice([]peer.AddrInfo{{ID: known, Addrs: []multiaddr.Multiaddr{addr}}, {ID: fresh, Addrs: []multiaddr.Multiaddr{addr}}}...)
		}, &out))

		var got lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, known.String()).Scan(&got))
		require.Equal(t, []string{"/ip4/10.0.0.7/tcp/4001"}, got.AddrList(), "addresses are refreshed")
		require.Equal(t, int64(8), got.Cores, "known resources are kept")
		require.True(t, got.Disabled, "operator settings are kept")

		require.NoError(t, lan.PeerFindByID(t.Context(), db, fresh.String()).Scan(&got))
		require.Equal(t, "discovered via mdns", got.Description)
		require.NotEmpty(t, got.PublicKey)
		require.Contains(t, out.String(), fresh.String())
	})

	t.Run("scan failures are reported", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		cmd := lanList{
			PeerTTL: lan.DefaultTTL,
			Scan:    true,
		}
		require.ErrorContains(t, cmd.list(gctx, db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			return iterx.Error[peer.AddrInfo](errors.New("no multicast"))
		}, &bytes.Buffer{}), "no multicast")
	})

	t.Run("runs against the given directory and writes to the context's stdout", func(t *testing.T) {
		dir := t.TempDir()

		db := testx.MustT(migrations.Open(t.Context(), filepath.Join(dir, migrations.CLI)))(t)
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("remembered"))).Scan(&lan.Peer{}))
		require.NoError(t, db.Close())

		priv, _, err := crypto.GenerateEd25519Key(nil)
		require.NoError(t, err)
		found, err := peer.IDFromPrivateKey(priv)
		require.NoError(t, err)

		var out bytes.Buffer
		genparser := cmdtestx.Genparser(lanList{},
			kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
			kong.Vars{"vars_cache_directory": t.TempDir()},
			kong.Writers(&out, nil),
			kong.Bind(libp2px.MDNSLookup(func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
				return iterx.Slice(peer.AddrInfo{ID: found})
			})),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--database", filepath.Join(dir, migrations.CLI), "--scan", "--scan-timeout", "10s", "--lan-peer-ttl", "48h"))
		require.Contains(t, out.String(), "remembered")
		require.Contains(t, out.String(), found.String(), "what the scan found is listed")
	})
}
