package cmdsecret

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/secrets"
)

type CmdEnv struct {
	URIs        []string      `name:"uri" short:"u" help:"Secret URIs containing KEY=VALUE environment variables. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Environment []string      `name:"env" short:"e" help:"define additional environment variables and their values, a bare key is read from the current environment. these override values from the secrets"`
	Grace       time.Duration `name:"grace" help:"how long the command is given to exit after eg is interrupted before it is killed, 0 waits indefinitely" default:"30s"`
	Cmd         []string      `arg:"" passthrough:"" optional:""`
}

func (t CmdEnv) Run(gctx *cmdopts.Global) error {
	secretvars, err := envx.FromReader(secrets.NewReader(gctx.Context, t.URIs...))
	if err != nil {
		return fmt.Errorf("failed to read secrets: %w", err)
	}

	secretvars = append(secretvars, envx.AutoEnviron(t.Environment...)...)

	if len(t.Cmd) == 0 {
		for _, v := range secretvars {
			fmt.Println(v)
		}
		return nil
	}

	cmd := exec.CommandContext(gctx.Context, t.Cmd[0], t.Cmd[1:]...)
	// when the context is cancelled (i.e. ctrl-c, which the terminal also delivers to the command) give the
	// command a chance to shut down gracefully instead of killing it: reporting os.ErrProcessDone leaves the
	// command alone and Run returns its own exit status. the grace period kills it if it fails to exit.
	cmd.Cancel = func() error { return os.ErrProcessDone }
	cmd.WaitDelay = t.Grace
	cmd.Env = append(os.Environ(), secretvars...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
