package compute

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/gofrs/uuid/v5"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestUploadLAN(t *testing.T) {
	cmd := upload{LANPeerTTL: lan.DefaultTTL}
	req := &computeapi.EnqueuedDequeueResponse{
		Enqueued: &computeapi.Enqueued{Id: uuid.Must(uuid.NewV7()).String(), AccountId: "acct", Entry: "main.wasm"},
	}

	t.Run("uploads to a remembered runner without the control plane's network", func(t *testing.T) {
		var token string

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			defer s.Close()

			r, err := http.ReadRequest(bufio.NewReader(s))
			if err != nil {
				return
			}

			var received computeapi.EnqueuedDequeueResponse
			require.NoError(t, json.Unmarshal([]byte(r.FormValue("enqueued")), &received))
			token = r.FormValue("token")

			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusAccepted)
			require.NoError(t, json.NewEncoder(rec).Encode(proto.Clone(received.Enqueued).(*computeapi.Enqueued)))

			_, _ = io.Copy(io.Discard, r.Body)
			_ = rec.Result().Write(s)
		})

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '10 days'`)

		candidates := []*compute.Compute{
			{Id: "stranger", P2Pid: "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN", Token: "stranger-token"},
			{Id: "nearby", P2Pid: runner.ID().String(), Token: "nearby-token"},
		}

		o, err := cmd.lan(t.Context(), db, candidates, req, strings.NewReader("kernel"), strings.NewReader("environ"))
		require.NoError(t, err)
		defer o.host.Close()

		require.Equal(t, "nearby", o.runner.Id)
		require.Equal(t, req.Enqueued.Id, o.recorded.Id)
		require.Equal(t, "nearby-token", token, "the control plane's candidate token authorizes the upload")

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, runner.ID().String()).Scan(&p))
		require.WithinDuration(t, time.Now(), p.LastSeen, time.Minute, "the accepting runner is kept fresh")
	})

	t.Run("reports when the runner declines", func(t *testing.T) {
		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			defer s.Close()

			r, err := http.ReadRequest(bufio.NewReader(s))
			if err != nil {
				return
			}

			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusConflict)
			_, _ = io.Copy(io.Discard, r.Body)
			_ = rec.Result().Write(s)
		})

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))

		_, err = cmd.lan(t.Context(), db, []*compute.Compute{{Id: "busy", P2Pid: runner.ID().String()}}, req, strings.NewReader("kernel"), strings.NewReader("environ"))
		require.ErrorContains(t, err, "no runner on the local network accepted")
	})

	t.Run("skips disabled runners", func(t *testing.T) {
		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))

		_, err = cmd.lan(t.Context(), db, []*compute.Compute{{Id: "disabled", P2Pid: runner.ID().String()}}, req, strings.NewReader("kernel"), strings.NewReader("environ"))
		require.ErrorContains(t, err, "known on the local network")
	})

	t.Run("skips expired runners", func(t *testing.T) {
		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()))).Scan(&lan.Peer{}))
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '90 days'`)

		_, err = cmd.lan(t.Context(), db, []*compute.Compute{{Id: "expired", P2Pid: runner.ID().String()}}, req, strings.NewReader("kernel"), strings.NewReader("environ"))
		require.ErrorContains(t, err, "known on the local network")
	})

	t.Run("runners without remembered addresses are skipped", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, lan.Peer{PeerID: "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN"}).Scan(&lan.Peer{}))

		_, err := cmd.lan(t.Context(), db, []*compute.Compute{{Id: "nowhere", P2Pid: "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN"}}, req, strings.NewReader("kernel"), strings.NewReader("environ"))
		require.ErrorContains(t, err, "known on the local network")
	})

	t.Run("fails without a cache", func(t *testing.T) {
		_, err := cmd.lan(t.Context(), nil, []*compute.Compute{{Id: "a", P2Pid: "b"}}, req, strings.NewReader("kernel"), strings.NewReader("environ"))
		require.ErrorContains(t, err, "cache is unavailable")
	})
}
