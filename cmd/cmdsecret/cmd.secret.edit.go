package cmdsecret

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/secrets"
)

type CmdEdit struct {
	URI string `arg:"" help:"Secret URI to edit. Example: chachasm://passphrase@/path/to/file"`
}

func (t CmdEdit) Run(gctx *cmdopts.Global) error {
	reader := secrets.Read(gctx.Context, t.URI)
	oldData, err := io.ReadAll(reader)
	if errorsx.Ignore(err, os.ErrNotExist) != nil {
		return fmt.Errorf("failed to read secret for editing: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "eg-secret-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(oldData); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	editor := envx.String("vi", "EDITOR")

	cmd := exec.CommandContext(gctx.Context, editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	newData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if bytes.Equal(oldData, newData) {
		fmt.Println("No changes detected; skipping update.")
		return nil
	}

	if err := secrets.Update(gctx.Context, t.URI, bytes.NewReader(newData)); err != nil {
		return fmt.Errorf("failed to update secret after edit: %w", err)
	}

	fmt.Println("Secret updated successfully.")
	return nil
}
