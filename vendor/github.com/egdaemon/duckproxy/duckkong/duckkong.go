// Package duckkong a kong command that opens a sql shell on a duckdb database, provided by usqlx (a fork of
// https://github.com/xo/usql), with psql style meta commands, i.e. \d.
package duckkong

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/egdaemon/duckproxy/usqlx/drivers"
	_ "github.com/egdaemon/duckproxy/usqlx/drivers/duckdb" // registers usqlx's duckdb support, the shell borrows it.
	"github.com/egdaemon/duckproxy/usqlx/drivers/metadata"
	"github.com/egdaemon/duckproxy/usqlx/env"
	"github.com/egdaemon/duckproxy/usqlx/handler"
	"github.com/egdaemon/duckproxy/usqlx/rline"
	"github.com/egdaemon/duckproxy/usqlx/text"
	"github.com/mattn/go-isatty"
	"github.com/xo/dburl"
)

// driver the name of the usqlx driver for databases opened with an Opener, the dsn is the path of the database.
const driver = "duckkong"

// Opener opens the database at path. it allows the shell to open the database the same way the rest of the
// application does, i.e. with locking, or through a duckproxy socket.
type Opener func(ctx context.Context, path string) (*sql.DB, error)

// DefaultOpener opens the path as a plain duckdb database.
func DefaultOpener(_ context.Context, path string) (*sql.DB, error) {
	return sql.Open("duckdb", path)
}

// registry the opener of the shell is chosen per run, usqlx's driver registry is global.
var registry struct {
	sync.Mutex
	open Opener
}

var register = sync.OnceFunc(func() {
	// the duckdb driver's metadata, completion, etc, with the connection opened by the Opener.
	d := drivers.Available()["duckdb"]
	d.Open = func(ctx context.Context, u *dburl.URL, _, _ func() io.Writer) (func(string, string) (*sql.DB, error), error) {
		return func(_ string, path string) (*sql.DB, error) {
			registry.Lock()
			open := registry.open
			registry.Unlock()
			return open(ctx, path)
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
	drivers.Register(driver, d)
})

// catalogs reads duckdb's catalogs, its attached databases, for the readers of usqlx's duckdb driver that don't
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

// Cmd opens a sql shell on the database, with psql style meta commands, i.e. \d, or runs the given statements and
// exits.
//
// the context and the Opener are provided by kong's bindings, i.e. kong.Bind(duckkong.Opener(open)), or
// kong.Bind(duckkong.Opener(duckkong.DefaultOpener)) for plain duckdb.
type Cmd struct {
	Path    string `name:"path" help:"path of the database, empty is resolved by the Opener"`
	Command string `name:"command" short:"c" help:"run the statements, or meta command, and exit instead of starting the shell"`
	// Prompt the prompt of the shell, %R and %# are replaced by usqlx.
	Prompt string `name:"prompt" help:"prompt of the shell" default:"duck%R%# "`
}

// Run the shell. open may be nil, see DefaultOpener.
func (t Cmd) Run(ctx context.Context, open Opener) (err error) {
	if open == nil {
		open = DefaultOpener
	}

	register()

	registry.Lock()
	registry.open = open
	registry.Unlock()

	// the driver has no short alias for the prompt's %S, which would show as (unk).
	if err = env.Set("PROMPT1", t.Prompt); err != nil {
		return fmt.Errorf("unable to set the prompt: %w", err)
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("unable to determine the current user: %w", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to determine the working directory: %w", err)
	}

	command := strings.TrimSpace(t.Command) != ""
	interactive := isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd())

	l, err := rline.New(interactive, false, command, "", env.HistoryFile(u))
	if err != nil {
		return fmt.Errorf("unable to start the shell: %w", err)
	}
	defer l.Close()

	h := handler.New(l, u, wd, nil, true)
	if err = h.Open(ctx, driver, t.Path); err != nil {
		return fmt.Errorf("unable to open the database %s: %w", t.Path, err)
	}
	defer h.Close()

	if command {
		h.SetSingleLineMode(true)
		h.Reset([]rune(t.Command))
	}

	return h.Run()
}
