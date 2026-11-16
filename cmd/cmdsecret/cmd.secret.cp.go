package cmdsecret

import (
	"bytes"
	"fmt"
	"io"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/secrets"
)

type CmdCp struct {
	Source      string `arg:"" help:"Secret URI to copy from. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Destination string `arg:"" help:"Secret URI to copy to, it can use a different scheme than the source."`
	Overwrite   bool   `name:"overwrite" help:"replace the destination when it already exists, by default an existing secret is never changed"`
}

func (t CmdCp) Run(gctx *cmdopts.Global) error {
	// the uris can carry the passphrase, so they are not repeated in errors.
	if !t.Overwrite {
		exists, err := secrets.Exists(gctx.Context, t.Destination)
		if err != nil {
			return fmt.Errorf("unable to check whether the destination exists: %w", err)
		}

		if exists {
			return fmt.Errorf("the destination already exists, use --overwrite to replace it")
		}
	}

	content, err := io.ReadAll(secrets.Read(gctx.Context, t.Source))
	if err != nil {
		return fmt.Errorf("failed to read the source secret: %w", err)
	}

	if err := secrets.Update(gctx.Context, t.Destination, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("failed to write the destination secret: %w", err)
	}

	return nil
}
