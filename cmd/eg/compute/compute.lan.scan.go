package compute

import (
	"database/sql"
	"io"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/libp2px"
)

type lanScan struct {
	CacheDir    string        `name:"cache-directory" help:"directory the remembered systems are stored in" default:"${vars_cache_directory}" env:"EG_CACHE_DIRECTORY"`
	ScanTimeout time.Duration `name:"scan-timeout" help:"how long to scan for" default:"3s"`
	PeerTTL     time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
}

func (t lanScan) Run(kctx *kong.Context, gctx *cmdopts.Global, find libp2px.MDNSLookup) (err error) {
	db, err := lan.Open(gctx.Context, filepath.Join(t.CacheDir, migrations.CLI), t.PeerTTL)
	if err != nil {
		return err
	}
	defer db.Close()

	return t.scan(gctx, db, find, kctx.Stdout)
}

// scan the network, remember what is found and list the remembered systems.
func (t lanScan) scan(gctx *cmdopts.Global, db *sql.DB, find libp2px.MDNSLookup, out io.Writer) (err error) {
	if err = lan.Discover(gctx.Context, db, find, t.ScanTimeout); err != nil {
		return err
	}

	return lanTable(gctx.Context, db, t.PeerTTL, out)
}
