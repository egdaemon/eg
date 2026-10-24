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

func TestLanScan(t *testing.T) {
	gctx := &cmdopts.Global{Context: t.Context()}

	t.Run("remembers new systems and keeps what is known about existing ones", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		priv1, _, err := crypto.GenerateEd25519Key(nil)
		require.NoError(t, err)
		known, err := peer.IDFromPrivateKey(priv1)
		require.NoError(t, err)

		priv2, _, err := crypto.GenerateEd25519Key(nil)
		require.NoError(t, err)
		fresh, err := peer.IDFromPrivateKey(priv2)
		require.NoError(t, err)

		addr := multiaddr.StringCast("/ip4/10.0.0.7/tcp/4001")

		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(known.String()))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(known.String()), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))

		var out bytes.Buffer
		cmd := lanScan{
			PeerTTL:     lan.DefaultTTL,
			ScanTimeout: time.Second,
		}
		require.NoError(t, cmd.scan(gctx, db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
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

	t.Run("scan failures are reported and nothing is listed", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		var out bytes.Buffer
		cmd := lanScan{
			PeerTTL: lan.DefaultTTL,
		}
		require.ErrorContains(t, cmd.scan(gctx, db, func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
			return iterx.Error[peer.AddrInfo](errors.New("no multicast"))
		}, &out), "no multicast")
		require.Empty(t, out.String())
	})

	t.Run("runs against the given directory and writes to the context's stdout", func(t *testing.T) {
		dir := t.TempDir()

		priv, _, err := crypto.GenerateEd25519Key(nil)
		require.NoError(t, err)
		found, err := peer.IDFromPrivateKey(priv)
		require.NoError(t, err)

		var out bytes.Buffer
		genparser := cmdtestx.Genparser(lanScan{},
			kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
			kong.Vars{"vars_cache_directory": t.TempDir()},
			kong.Writers(&out, nil),
			kong.Bind(libp2px.MDNSLookup(func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
				return iterx.Slice(peer.AddrInfo{ID: found})
			})),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "--cache-directory", dir, "--scan-timeout", "10s", "--lan-peer-ttl", "48h"))
		require.Contains(t, out.String(), found.String())

		db := testx.MustT(migrations.Open(t.Context(), filepath.Join(dir, migrations.CLI)))(t)
		defer db.Close()
		require.NoError(t, lan.PeerFindByID(t.Context(), db, found.String()).Scan(&lan.Peer{}), "remembered in the given directory")
	})

	t.Run("only applies to the client database", func(t *testing.T) {
		genparser := cmdtestx.Genparser(lanScan{}, kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)), kong.Vars{"vars_cache_directory": t.TempDir()})

		_, err := genparser(t).Parse([]string{"command", "--runner"})
		require.Error(t, err)
	})
}
