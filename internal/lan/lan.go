// Package lan caches the eg runners seen on the local network (client side) and the peers
// that reached a runner (runner side). entries expire so stale machines age out, see DefaultTTL.
package lan

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	_ "github.com/duckdb/duckdb-go/v2" // registers the duckdb driver.
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/goosex"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/uuid/v5"
)

//go:embed migrations
var migrationsdir embed.FS

// migrations rooted at the directory's contents, goose expects migration filenames at the fs root.
var migrations = errorsx.Must(fs.Sub(migrationsdir, "migrations"))

// DefaultTTL is how long a peer is remembered after it was last seen.
const DefaultTTL = 30 * 24 * time.Hour

// Role selects which database file is used. duckdb is single writer per file so a daemon and
// a cli sharing a cache directory must not share a file.
type Role string

const (
	RoleRunner Role = "runner"
	RoleClient Role = "client"
)

// Path is the location of the database for the role within the cache directory,
// CACHE_DIRECTORY (systemd) first then XDG_CACHE_HOME, see userx.DefaultCacheDirectory.
func Path(r Role) string {
	return PathIn(userx.DefaultCacheDirectory(), r)
}

// PathIn is the location of the database for the role within the given cache directory.
func PathIn(dir string, r Role) string {
	return filepath.Join(dir, "lan."+string(r)+".duckdb")
}

// Migrate applies the schema to an already opened database.
func Migrate(ctx context.Context, db *sql.DB) error {
	return goosex.Up(ctx, db, migrations)
}

// Open the database at path, applying migrations and purging peers older than ttl.
// see Path for the conventional location of the role's database.
func Open(ctx context.Context, path string, ttl time.Duration) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create lan peers directory")
	}

	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to open lan peers database")
	}

	if err = Migrate(ctx, db); err != nil {
		errorsx.Log(db.Close())
		return nil, err
	}

	// purge the expired.
	if err = sqlx.Discard(sqlx.Scan(PeerPurgeExpired(ctx, db, int64(ttl)))); err != nil {
		errorsx.Log(db.Close())
		return nil, errorsx.Wrap(err, "unable to purge expired lan peers")
	}

	return db, nil
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
