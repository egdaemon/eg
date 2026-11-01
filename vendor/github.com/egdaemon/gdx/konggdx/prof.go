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

// Prof dials a gdx debug socket and streams a profile for the given mode.
type Prof struct {
	Mode     string        `arg:"" name:"mode" help:"profile mode" enum:"cpu,heap,mem,allocs,block"`
	Socket   string        `flag:"" name:"socket" help:"unix socket path to dial" default:"${vars_gdx_socket}"`
	Duration time.Duration `flag:"" name:"duration" help:"length of the capture" default:"30s"`
	Output   cmdopts.IOOut `flag:"" name:"output" help:"output destination; '-' for stdout" default:"${vars_gdx_default_output}"`
}

func (t Prof) Run(ctx context.Context) error {
	out, err := t.Output.Open(os.Stdout)
	if err != nil {
		return fmt.Errorf("unable to open output: %w", err)
	}
	defer out.Close()

	return t.run(ctx, gdxapi.NewUnixClient(t.Socket), out)
}

func (t Prof) run(ctx context.Context, c *http.Client, out io.Writer) error {
	return gdxapi.Fetch(ctx, c, "/debug/profile/"+t.Mode, t.Duration, out)
}
