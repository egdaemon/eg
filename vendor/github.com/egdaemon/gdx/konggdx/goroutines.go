package konggdx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/egdaemon/gdx/gdxapi"
	"github.com/egdaemon/gdx/konggdx/cmdopts"
)

// Goroutines dials a gdx debug socket and dumps the current goroutine stack traces.
type Goroutines struct {
	Socket string        `flag:"" name:"socket" help:"unix socket path to dial" default:"${vars_gdx_socket}"`
	Output cmdopts.IOOut `flag:"" name:"output" help:"output destination; '-' for stdout" default:"${vars_gdx_default_output}"`
}

func (t Goroutines) Run(ctx context.Context) error {
	return t.Do(ctx, gdxapi.NewUnixClient(t.Socket))
}

// Do fetches the goroutine dump through c, writing it to the configured output. Socket is ignored, allowing
// callers to supply a client for another transport (see gdxapi.NewClient).
func (t Goroutines) Do(ctx context.Context, c *http.Client) error {
	out, err := t.Output.Open(os.Stdout)
	if err != nil {
		return fmt.Errorf("unable to open output: %w", err)
	}
	defer out.Close()

	return t.run(ctx, c, out)
}

func (t Goroutines) run(ctx context.Context, c *http.Client, out io.Writer) error {
	return gdxapi.Fetch(ctx, c, "/debug/goroutines", 0, out)
}
