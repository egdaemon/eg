// Package cmddb a sql shell for the cli's local duckdb database, provided by usql, see https://github.com/xo/usql.
package cmddb

import (
	"context"
	"database/sql"
	"io"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/mattn/go-isatty"
	"github.com/xo/dburl"
	"github.com/xo/usql/drivers"
	_ "github.com/xo/usql/drivers/duckdb" // registers usql's duckdb support, the eg driver borrows it.
	"github.com/xo/usql/drivers/metadata"
	"github.com/xo/usql/env"
	"github.com/xo/usql/handler"
	"github.com/xo/usql/rline"
	"github.com/xo/usql/text"
)

// usqldriver the name of the usql driver for databases opened with migrations.OpenLocked, the dsn is the path of the database.
const usqldriver = "egduckdb"

var registerusql = sync.OnceFunc(func() {
	// the duckdb driver's metadata, completion, etc, with the connection opened the way the other commands open it.
	d := drivers.Available()["duckdb"]
	d.Open = func(ctx context.Context, u *dburl.URL, _, _ func() io.Writer) (func(string, string) (*sql.DB, error), error) {
		return func(_ string, path string) (*sql.DB, error) {
			return migrations.OpenLocked(ctx, path)
		}, nil
	}
	// the duckdb driver uses the mysql completer, which queries DATABASE() and information_schema.routines that
	// duckdb lacks. the default completer uses the driver's metadata reader instead.
	d.NewCompleter = nil
	// it lists catalogs with SHOW catalogs and table privileges from information_schema.table_privileges, which
	// duckdb lacks.
	reader := d.NewMetadataReader
	d.NewMetadataReader = func(db drivers.DB, opts ...metadata.ReaderOption) metadata.Reader {
		// the later readers take precedence.
		return metadata.NewPluginReader(reader(db, opts...), catalogs{db: db})
	}
	// the duckdb driver's writer, i.e. \l, uses its own reader.
	d.NewMetadataWriter = func(db drivers.DB, w io.Writer, opts ...metadata.ReaderOption) metadata.Writer {
		return metadata.NewDefaultWriter(d.NewMetadataReader(db, opts...))(db, w)
	}
	drivers.Register(usqldriver, d)
})

// catalogs reads duckdb's catalogs, its attached databases, for the readers of usql's duckdb driver that don't
// work with duckdb.
type catalogs struct {
	db drivers.DB
}

func (t catalogs) Catalogs(metadata.Filter) (_ *metadata.CatalogSet, err error) {
	rows, err := t.db.Query(`SELECT database_name FROM duckdb_databases() WHERE NOT internal ORDER BY database_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []metadata.Catalog{}
	for rows.Next() {
		var c metadata.Catalog
		if err = rows.Scan(&c.Catalog); err != nil {
			return nil, err
		}
		results = append(results, c)
	}

	return metadata.NewCatalogSet(results), rows.Err()
}

// PrivilegeSummaries reports the privileges, i.e. \dp, as not supported, duckdb lacks
// information_schema.table_privileges.
func (t catalogs) PrivilegeSummaries(metadata.Filter) (*metadata.PrivilegeSummarySet, error) {
	return nil, text.ErrNotSupported
}

// Cmd opens a sql shell on the database, with usql's psql style meta commands, i.e. \d, or runs the given statements
// and exits. the database is opened the same way the other commands open it, so it works while other eg commands have
// it open, see migrations.OpenLocked.
type Cmd struct {
	Path    string `name:"path" help:"path of the database" default:"${vars_cache_directory}/${vars_cli_database}"`
	Command string `name:"command" short:"c" help:"run the statements, or meta command, and exit instead of starting the shell"`
}

func (t Cmd) Run(gctx *cmdopts.Global) (err error) {
	registerusql()

	// the driver has no short alias for the prompt's %S, which would show as (unk).
	if err = env.Set("PROMPT1", "eg%R%# "); err != nil {
		return errorsx.Wrap(err, "unable to set the prompt")
	}

	u, err := user.Current()
	if err != nil {
		return errorsx.Wrap(err, "unable to determine the current user")
	}

	wd, err := os.Getwd()
	if err != nil {
		return errorsx.Wrap(err, "unable to determine the working directory")
	}

	command := strings.TrimSpace(t.Command) != ""
	interactive := isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd())

	l, err := rline.New(interactive, false, command, "", env.HistoryFile(u))
	if err != nil {
		return errorsx.Wrap(err, "unable to start the shell")
	}
	defer l.Close()

	h := handler.New(l, u, wd, nil, true)
	if err = h.Open(gctx.Context, usqldriver, t.Path); err != nil {
		return errorsx.Wrapf(err, "unable to open the database %s", t.Path)
	}
	defer h.Close()

	if command {
		h.SetSingleLineMode(true)
		h.Reset([]rune(t.Command))
	}

	return h.Run()
}
