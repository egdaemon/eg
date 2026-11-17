package cmdsecret

import (
	"fmt"
	"log"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/secrets"
)

type CmdRm struct {
	URIs         []string `arg:"" help:"Secret URIs to remove. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name, awssm://secret-name?region=us-east-1"`
	AllowMissing bool     `name:"allow-missing" help:"skip secrets that do not exist, logging a warning instead of failing"`
}

func (t CmdRm) Run(gctx *cmdopts.Global) error {
	// the uris can carry the passphrase, so they are not repeated in errors.
	for _, uri := range t.URIs {
		exists, err := secrets.Exists(gctx.Context, uri)
		if err != nil {
			return fmt.Errorf("unable to check whether the secret exists: %w", err)
		}

		if !exists {
			if t.AllowMissing {
				log.Println("WARN", "the secret does not exist, skipping")
				continue
			}

			return fmt.Errorf("the secret does not exist, use --allow-missing to skip it")
		}

		if err := secrets.Delete(gctx.Context, uri); err != nil {
			return fmt.Errorf("failed to remove secret: %w", err)
		}
	}

	return nil
}
