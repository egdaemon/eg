//go:build genieql.generate

package lan

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func Peer(gql genieql.Structure) {
	gql.From(gql.Table("lan_peers"))
}

func PeerScanner(gql genieql.Scanner, pattern func(p Peer)) {}

// PeerUpsert records a sighting. disabled, expires_after and first_seen are preserved on conflict.
func PeerUpsert(
	gql genieql.Insert,
	pattern func(ctx context.Context, q sqlx.Queryer, p Peer) NewPeerScannerStaticRow,
) {
	gql.Into("lan_peers").Default("first_seen", "last_seen", "expires_after", "disabled").Conflict(
		"ON CONFLICT (peer_id) DO UPDATE SET last_seen = now(), public_key = EXCLUDED.public_key, addrs = EXCLUDED.addrs, description = EXCLUDED.description, os = EXCLUDED.os, arch = EXCLUDED.arch, cores = EXCLUDED.cores, memory = EXCLUDED.memory, disk = EXCLUDED.disk, vram = EXCLUDED.vram, workload_ttl = EXCLUDED.workload_ttl, labels = EXCLUDED.labels, uploadable = EXCLUDED.uploadable",
	)
}

func PeerFindByID(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, id string) NewPeerScannerStaticRow,
) {
	gql = gql.Query(`SELECT ` + PeerScannerStaticColumns + ` FROM lan_peers WHERE peer_id = {id}`)
}

// PeerLookupLive returns the peers that haven't expired, most recently seen first.
// a peer is live when seen within its own expires_after, or the default ttl (nanoseconds) when unset; negative never expires.
// the predicate is repeated inline because genieql only carries function bodies into its generation program.
func PeerLookupLive(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, ttl int64) NewPeerScannerStatic,
) {
	live := `(expires_after < 0 OR last_seen + to_microseconds(CAST((CASE WHEN expires_after > 0 THEN expires_after ELSE {ttl} END) / 1000 AS BIGINT)) > now())`
	gql = gql.Query(`SELECT ` + PeerScannerStaticColumns + ` FROM lan_peers WHERE ` + live + ` ORDER BY last_seen DESC`)
}

// PeerLookupAll returns every peer including expired ones, most recently seen first.
func PeerLookupAll(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer) NewPeerScannerStatic,
) {
	gql = gql.Query(`SELECT ` + PeerScannerStaticColumns + ` FROM lan_peers ORDER BY last_seen DESC`)
}

// PeerUpdate modifies the operator controlled fields of a peer.
func PeerUpdate(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, p Peer) NewPeerScannerStaticRow,
) {
	gql = gql.Query(`UPDATE lan_peers SET description = {p.Description}, disabled = {p.Disabled}, expires_after = {p.ExpiresAfter} WHERE peer_id = {p.PeerID} RETURNING ` + PeerScannerStaticColumns)
}

// PeerTouch refreshes last_seen, keeping a successfully used peer alive.
func PeerTouch(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, id string) NewPeerScannerStaticRow,
) {
	gql = gql.Query(`UPDATE lan_peers SET last_seen = now() WHERE peer_id = {id} RETURNING ` + PeerScannerStaticColumns)
}

func PeerDeleteByID(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, id string) NewPeerScannerStaticRow,
) {
	gql = gql.Query(`DELETE FROM lan_peers WHERE peer_id = {id} RETURNING ` + PeerScannerStaticColumns)
}

// PeerPurgeExpired deletes and returns the peers that have expired.
func PeerPurgeExpired(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, ttl int64) NewPeerScannerStatic,
) {
	live := `(expires_after < 0 OR last_seen + to_microseconds(CAST((CASE WHEN expires_after > 0 THEN expires_after ELSE {ttl} END) / 1000 AS BIGINT)) > now())`
	gql = gql.Query(`DELETE FROM lan_peers WHERE NOT ` + live + ` RETURNING ` + PeerScannerStaticColumns)
}
