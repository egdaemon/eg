package konggdx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/egdaemon/gdx/gdxapi"
	"github.com/egdaemon/gdx/konggdx/cmdopts"
)

// Trace dials a gdx debug socket and streams a runtime/trace execution trace.
type Trace struct {
	Socket   string        `flag:"" name:"socket" help:"unix socket path to dial" default:"${vars_gdx_socket}"`
	Duration time.Duration `flag:"" name:"duration" help:"length of the trace capture" default:"30s"`
	Output   cmdopts.IOOut `flag:"" name:"output" help:"output destination; '-' for stdout" default:"-"`
}

func (t Trace) Run(ctx context.Context) error {
	out, err := t.Output.Open(os.Stdout)
	if err != nil {
		return fmt.Errorf("unable to open output: %w", err)
	}
	defer out.Close()

	return t.run(ctx, gdxapi.NewUnixClient(t.Socket), out)
}

func (t Trace) run(ctx context.Context, c *http.Client, out io.Writer) error {
	return gdxapi.Fetch(ctx, c, "/debug/trace", t.Duration, out)
}
