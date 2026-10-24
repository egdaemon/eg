// Package lan caches the eg runners seen on the local network (client side) and the peers
// that reached a runner (runner side). entries expire so stale machines age out, see DefaultTTL.
package lan

import (
	"context"
	"encoding/json"
	"time"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/gofrs/uuid/v5"
)

// DefaultTTL is how long a peer is remembered after it was last seen.
const DefaultTTL = 30 * 24 * time.Hour

// Purge deletes the peers that have expired, see PeerLookupLive for when a peer has.
func Purge(ctx context.Context, db sqlx.Queryer, ttl time.Duration) error {
	return errorsx.Wrap(sqlx.Discard(sqlx.Scan(PeerPurgeExpired(ctx, db, int64(ttl)))), "unable to purge expired lan peers")
}

// PeerOptionTestDefaults populates a peer with representative values and a unique id,
// intended for use with langx.Clone in tests.
func PeerOptionTestDefaults(p *Peer) {
	p.PeerID = uuid.Must(uuid.NewV7()).String()
	p.PublicKey = []byte{1, 2, 3}
	p.Addrs = EncodeList([]string{"/ip4/192.168.1.2/tcp/4001"})
	p.Description = "host - " + p.PeerID
	p.OS = "linux"
	p.Arch = "amd64"
	p.Cores = 8
	p.Memory = 16 * bytesx.GiB
	p.Disk = 100 * bytesx.GiB
	p.Vram = 4 * bytesx.GiB
	p.WorkloadTTL = int64(time.Hour)
	p.Labels = EncodeList([]string{"gpu", "fast"})
	p.Uploadable = true
}

// PeerOptionID sets the peer's id.
func PeerOptionID(id string) func(*Peer) {
	return func(p *Peer) {
		p.PeerID = id
	}
}

// AddrList decodes the peer's stored multiaddrs.
func (t Peer) AddrList() (zero []string) {
	return decodelist(t.Addrs)
}

// LabelList decodes the peer's stored labels.
func (t Peer) LabelList() (zero []string) {
	return decodelist(t.Labels)
}

// EncodeList encodes a list of strings in the form stored in the addrs and labels columns.
func EncodeList(l []string) string {
	if l == nil {
		l = []string{}
	}

	return string(errorsx.Must(json.Marshal(l)))
}

func decodelist(s string) (zero []string) {
	var l []string
	if err := json.Unmarshal([]byte(s), &l); err != nil {
		return zero
	}

	return l
}
