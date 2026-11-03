package cmdsecret

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/ttyx"
)

// runeditor opens the file in $EDITOR on the terminal the user types on, and waits for it to close.
func runeditor(ctx context.Context, path string) error {
	editor := envx.String("vi", "EDITOR")

	// the user types on the terminal, stdin may have been consumed as input.
	editorin := ttyx.Input(ctx)
	defer editorin.Close()

	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = editorin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	return nil
}
