package browserx

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/egdaemon/eg/internal/errorsx"
)

// Open the url in the default browser. the browser is detached, it is not waited on.
func Open(ctx context.Context, url string) (err error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}

	if err = cmd.Start(); err != nil {
		return errorsx.Wrap(err, "unable to open the browser")
	}

	return errorsx.Wrap(cmd.Process.Release(), "unable to release the browser process")
}
