package compute

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"
	_ "github.com/egdaemon/eg/duckproxy" // registers the duckproxy driver.
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/internal/userx"
)

type lanCmd struct {
	List   lanList   `cmd:"" name:"ls" help:"list the systems remembered on the local network"`
	Scan   lanScan   `cmd:"" name:"scan" help:"scan the local network for systems and remember them"`
	Update lanUpdate `cmd:"" name:"upd" help:"update a remembered system, i.e. disable it or adjust how long it is remembered"`
	Remove lanRemove `cmd:"" name:"rm" help:"forget a remembered system"`
}

// openlan opens the database of the lan peers at the path, defaulting to the client's. the daemon holds its own
// database open, duckdb only allows a single process to have a file open, so it is reached through the daemon
// when it is running. expired peers are purged.
func openlan(ctx context.Context, database string, ttl time.Duration) (db *sql.DB, err error) {
	if database == "" {
		database = migrations.Path(migrations.CLI)
	}

	if socket := userx.DefaultRuntimeDirectory(migrations.DatabaseSocketName); filepath.Base(database) == migrations.Daemon && fsx.FileExists(socket) {
		db, err = sql.Open("duckproxy", socket)
	} else {
		db, err = migrations.Open(ctx, database)
	}

	if err != nil {
		return nil, err
	}

	if err = lan.Purge(ctx, db, ttl); err != nil {
		errorsx.Log(db.Close())
		return nil, err
	}

	return db, nil
}

// lanTable prints the remembered, unexpired systems.
func lanTable(ctx context.Context, db *sql.DB, ttl time.Duration, out io.Writer) (err error) {
	var peers []lan.Peer

	if err = sqlx.ScanInto(lan.PeerLookupLive(ctx, db, int64(ttl)), &peers); err != nil {
		return errorsx.Wrap(err, "unable to retrieve remembered systems")
	}

	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDESCRIPTION\tOS/ARCH\tCORES\tMEMORY\tDISK\tVRAM\tLABELS\tLAST SEEN\tEXPIRES\tSTATE")
	for _, p := range peers {
		fmt.Fprintf(
			w, "%s\t%s\t%s/%s\t%d\t%v\t%v\t%v\t%s\t%s\t%s\t%s\n",
			p.PeerID, p.Description, p.OS, p.Arch, p.Cores, bytesx.Unit(p.Memory), bytesx.Unit(p.Disk), bytesx.Unit(p.Vram),
			strings.Join(p.LabelList(), ","), p.LastSeen.Local().Format(time.DateTime), expires(p, ttl), state(p),
		)
	}

	return w.Flush()
}

// expires describes when the system will be forgotten.
func expires(p lan.Peer, ttl time.Duration) string {
	switch {
	case p.ExpiresAfter < 0:
		return "never"
	case p.ExpiresAfter > 0:
		return p.LastSeen.Add(time.Duration(p.ExpiresAfter)).Local().Format(time.DateTime)
	default:
		return p.LastSeen.Add(ttl).Local().Format(time.DateTime)
	}
}

func state(p lan.Peer) string {
	if p.Disabled {
		return "disabled"
	}

	return ""
}
