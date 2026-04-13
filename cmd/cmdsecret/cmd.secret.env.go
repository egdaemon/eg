package cmdsecret

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/secrets"
)

type CmdEnv struct {
	URIs []string `name:"uri" short:"u" help:"Secret URIs containing KEY=VALUE environment variables. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Cmd  []string `arg:"" passthrough:"" optional:""`
}

func (t CmdEnv) Run(gctx *cmdopts.Global) error {
	secretvars, err := envx.FromReader(secrets.NewReader(gctx.Context, t.URIs...))
	if err != nil {
		return fmt.Errorf("failed to read secrets: %w", err)
	}

	if len(t.Cmd) == 0 {
		for _, v := range secretvars {
			fmt.Println(v)
		}
		return nil
	}

	cmd := exec.CommandContext(gctx.Context, t.Cmd[0], t.Cmd[1:]...)
	cmd.Env = append(os.Environ(), secretvars...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
