package compute

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/sqlx"
)

type lanCmd struct {
	List   lanList   `cmd:"" name:"ls" help:"list the systems remembered on the local network"`
	Scan   lanScan   `cmd:"" name:"scan" help:"scan the local network for systems and remember them"`
	Update lanUpdate `cmd:"" name:"upd" help:"update a remembered system, i.e. disable it or adjust how long it is remembered"`
	Remove lanRemove `cmd:"" name:"rm" help:"forget a remembered system"`
}

// lanDatabase are the options shared by the commands that operate on the lan peer database.
type lanDatabase struct {
	CacheDir string        `name:"cache-directory" help:"directory the remembered systems are stored in" default:"${vars_cache_directory}" env:"EG_CACHE_DIRECTORY"`
	Runner   bool          `name:"runner" help:"operate on the daemon's database (who has reached it) instead of the client's (the systems it can reach)"`
	PeerTTL  time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
}

func (t lanDatabase) role() lan.Role {
	if t.Runner {
		return lan.RoleRunner
	}

	return lan.RoleClient
}

func (t lanDatabase) open(gctx *cmdopts.Global) (*sql.DB, error) {
	return lan.Open(gctx.Context, lan.PathIn(t.CacheDir, t.role()), t.PeerTTL)
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
