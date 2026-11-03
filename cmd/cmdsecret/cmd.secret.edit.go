package cmdsecret

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/ttyx"
	"github.com/egdaemon/eg/secrets"
)

type CmdEdit struct {
	URI string `arg:"" help:"Secret URI to edit. examples: file:./relative/path, file:///absolute/path, chachasm://passphrase@/path/to/file"`
}

func (t CmdEdit) Run(gctx *cmdopts.Global) error {
	fbuf, err := os.CreateTemp("", "eg.secret.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(fbuf.Name())
	defer fbuf.Close()

	// the editor is shown the secret, followed by anything piped on stdin. the secret is streamed into the
	// file while its md5 is taken, which is all that is needed later to know whether the editor changed it.
	var (
		stored = md5.New()
		tail   = iox.NewTail(1)
	)
	_, err = io.Copy(io.MultiWriter(fbuf, stored, tail), secrets.Read(gctx.Context, t.URI))
	if errorsx.Ignore(err, os.ErrNotExist) != nil {
		return fmt.Errorf("failed to read secret for editing: %w", err)
	}

	// nothing is read when stdin is a terminal, the user is going to type.
	piped := ttyx.Read(gctx.Context)
	defer piped.Close()
	if _, err = io.Copy(fbuf, iox.SpacerFrom(tail.Last(), piped)); err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	if err = fbuf.Close(); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err := runeditor(gctx.Context, fbuf.Name()); err != nil {
		return err
	}

	edited, err := os.Open(fbuf.Name())
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}
	defer edited.Close()

	result := md5.New()
	if _, err = io.Copy(result, edited); err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if bytes.Equal(stored.Sum(nil), result.Sum(nil)) {
		fmt.Println("No changes detected; skipping update.")
		return nil
	}

	if _, err = edited.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if err := secrets.Update(gctx.Context, t.URI, edited); err != nil {
		return fmt.Errorf("failed to update secret after edit: %w", err)
	}

	fmt.Println("Secret updated successfully.")
	return nil
}
