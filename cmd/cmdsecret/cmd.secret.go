package cmdsecret

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/ttyx"
	"github.com/egdaemon/eg/secrets"
)

type SecretCmd struct {
	Read   CmdRead   `cmd:"" help:"Read secrets from various schemes."`
	Create CmdCreate `cmd:"" help:"Create or replace secrets for various schemes."`
	Edit   CmdEdit   `cmd:"" help:"Interactively edit a secret using $EDITOR."`
	B64    CmdB64    `cmd:"" name:"b64" help:"Base64 URL encode stdin and write the result to stdout or a file."`
	Env    CmdEnv    `cmd:"" name:"env" help:"Run a command with environment variables loaded from secret URIs."`
}

type CmdRead struct {
	URIs         []string `arg:"" help:"List of secret URIs to download. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Output       string   `name:"output" short:"o" help:"Write output to a file instead of stdout"`
	AllowMissing bool     `name:"allow-missing" help:"skip secrets that do not exist, logging a warning instead of failing"`
}

func (t CmdRead) Run(gctx *cmdopts.Global) error {
	var out io.Writer = os.Stdout

	uris := t.URIs
	if t.AllowMissing {
		uris = make([]string, 0, len(t.URIs))
		for _, uri := range t.URIs {
			exists, err := secrets.Exists(gctx.Context, uri)
			if err != nil {
				return fmt.Errorf("unable to check whether the secret exists: %w", err)
			}

			if !exists {
				// the uri can carry the passphrase, so it is not repeated.
				log.Println("WARN", "the secret does not exist, skipping")
				continue
			}

			uris = append(uris, uri)
		}
	}

	if t.Output != "" {
		f, err := os.Create(t.Output)
		if err != nil {
			return fmt.Errorf("unable to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	if _, err := io.Copy(out, secrets.NewReader(gctx.Context, uris...)); err != nil {
		return fmt.Errorf("failed to read secrets: %w", err)
	}

	return nil
}

type CmdCreate struct {
	URI        string `arg:"" help:"Secret URI to create or replace. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name, awssm://secret-name?region=us-east-1"`
	Input      string `name:"input" short:"i" help:"Read content from a file instead of stdin"`
	Overwrite  bool   `name:"overwrite" help:"replace the secret when it already exists, by default an existing secret is never changed"`
	AllowEmpty bool   `name:"allow-empty" help:"allow an empty secret to be written when there is no input from stdin or the editor"`
	TTY        string `name:"tty" help:"terminal device to open the editor on instead of the controlling terminal, a path that does not exist means no editor" env:"EG_TTY"`
}

func (t CmdCreate) Run(gctx *cmdopts.Global) error {
	if !t.Overwrite {
		exists, err := secrets.Exists(gctx.Context, t.URI)
		if err != nil {
			return fmt.Errorf("unable to check whether the secret exists: %w", err)
		}

		if exists {
			// the uri can carry the passphrase, so it is not repeated.
			return fmt.Errorf("the secret already exists, use --overwrite to replace it")
		}
	}

	if t.Input != "" {
		f, err := os.Open(t.Input)
		if err != nil {
			return fmt.Errorf("unable to open input file: %w", err)
		}
		defer f.Close()

		if err := secrets.Update(gctx.Context, t.URI, f); err != nil {
			return fmt.Errorf("failed to create secret [%s]: %w", t.URI, err)
		}

		return nil
	}

	ttyx.SetPath(t.TTY)

	// nothing is read when stdin is a terminal, the user is going to type.
	piped := ttyx.Read(gctx.Context)
	defer piped.Close()
	content, err := io.ReadAll(piped)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	// with a terminal the user gets to edit what was piped, without one the piped content is the secret.
	if ttyx.Available() {
		fbuf, err := os.CreateTemp("", "eg.secret.*")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(fbuf.Name())
		defer fbuf.Close()

		if _, err = fbuf.Write(content); err != nil {
			return fmt.Errorf("failed to write to temp file: %w", err)
		}

		if err = fbuf.Close(); err != nil {
			return fmt.Errorf("failed to write to temp file: %w", err)
		}

		if err = runeditor(gctx.Context, fbuf.Name()); err != nil {
			return err
		}

		if content, err = os.ReadFile(fbuf.Name()); err != nil {
			return fmt.Errorf("failed to read edited file: %w", err)
		}
	}

	if len(content) == 0 && !t.AllowEmpty {
		return fmt.Errorf("no secret content provided from stdin or the editor, use --allow-empty to write an empty secret")
	}

	if err := secrets.Update(gctx.Context, t.URI, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("failed to create secret [%s]: %w", t.URI, err)
	}

	return nil
}

type CmdB64 struct {
	Output string `name:"output" short:"o" help:"Write output to a file instead of stdout"`
}

func (t CmdB64) Run(gctx *cmdopts.Global) error {
	var out io.Writer = os.Stdout

	if t.Output != "" {
		f, err := os.Create(t.Output)
		if err != nil {
			return fmt.Errorf("unable to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	_, err = fmt.Fprint(out, base64.URLEncoding.EncodeToString(raw))
	return err
}
