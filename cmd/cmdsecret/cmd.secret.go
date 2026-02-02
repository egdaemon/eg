package cmdsecret

import (
	"fmt"
	"io"
	"os"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/secrets"
)

type SecretCmd struct {
	Read   CmdRead   `cmd:"" help:"Read secrets from various schemes."`
	Update CmdUpdate `cmd:"" help:"Update or create secrets for various schemes."`
	Edit   CmdEdit   `cmd:"" help:"Interactively edit a secret using $EDITOR."`
}

type CmdRead struct {
	URIs   []string `arg:"" help:"List of secret URIs to download. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Output string   `name:"output" short:"o" help:"Write output to a file instead of stdout"`
}

func (t CmdRead) Run(gctx *cmdopts.Global) error {
	var out io.Writer = os.Stdout

	if t.Output != "" {
		f, err := os.Create(t.Output)
		if err != nil {
			return fmt.Errorf("unable to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	for _, uri := range t.URIs {
		if _, err := io.Copy(out, secrets.Read(gctx.Context, uri)); err != nil {
			return fmt.Errorf("failed to read secret [%s]: %w", uri, err)
		}

		// add a new line after each.
		if _, err := out.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to read secret [%s]: %w", uri, err)
		}
	}

	return nil
}

type CmdUpdate struct {
	URI   string `arg:"" help:"Secret URI to update. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name, awssm://secret-name?region=us-east-1"`
	Input string `name:"input" short:"i" help:"Read content from a file instead of stdin"`
}

func (t CmdUpdate) Run(gctx *cmdopts.Global) error {
	var in io.Reader = os.Stdin

	if t.Input != "" {
		f, err := os.Open(t.Input)
		if err != nil {
			return fmt.Errorf("unable to open input file: %w", err)
		}
		defer f.Close()
		in = f
	}

	if err := secrets.Update(gctx.Context, t.URI, in); err != nil {
		return fmt.Errorf("failed to update secret [%s]: %w", t.URI, err)
	}

	return nil
}
