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
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/goosex"
	"github.com/egdaemon/eg/internal/userx"
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
	return userx.DefaultCacheDirectory(filepath.Join("lan." + string(r) + ".duckdb"))
}

// Open the database for the role, applying migrations and purging expired peers.
func Open(ctx context.Context, r Role, ttl time.Duration) (*sql.DB, error) {
	path := Path(r)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create lan peers directory")
	}

	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to open lan peers database")
	}

	if err = goosex.Up(ctx, db, migrations); err != nil {
		errorsx.Log(db.Close())
		return nil, err
	}

	// purge the expired. iterate the returned rows so the delete is fully applied.
	scanner := PeerPurgeExpired(ctx, db, int64(ttl))
	defer scanner.Close()
	for scanner.Next() {
		var p Peer
		if err = scanner.Scan(&p); err != nil {
			break
		}
	}

	return db, errorsx.Wrap(scanner.Err(), "unable to purge expired lan peers")
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
