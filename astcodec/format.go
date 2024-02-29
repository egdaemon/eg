package astcodec

import (
	"go/format"
	"io"

	"github.com/egdaemon/eg/internal/iox"
	"github.com/pkg/errors"
	"golang.org/x/tools/imports"
)

type formattableio interface {
	io.ReadWriteSeeker
	Truncate(size int64) error
}

// Reformat a seekable io stream.
func Reformat(in formattableio) (err error) {
	var (
		raw []byte
	)

	// ensure we're at the start of the file.
	if err = iox.Rewind(in); err != nil {
		return err
	}

	if raw, err = io.ReadAll(in); err != nil {
		return err
	}

	if raw, err = imports.Process("generated.go", []byte(string(raw)), nil); err != nil {
		return errors.Wrap(err, "failed to add required imports")
	}

	// ensure we're at the start of the file.
	if err = iox.Rewind(in); err != nil {
		return err
	}

	if err = in.Truncate(0); err != nil {
		return errors.Wrap(err, "failed to truncate file")
	}

	if _, err = in.Write(raw); err != nil {
		return errors.Wrap(err, "failed to write formatted content")
	}

	return nil
}

// Format arbitrary source fragment.
func Format(s string) (_ string, err error) {
	var (
		raw []byte
	)

	if raw, err = imports.Process("generated.go", []byte(s), &imports.Options{Fragment: true, Comments: true, TabIndent: true, TabWidth: 8}); err != nil {
		return "", errors.Wrap(err, "failed to add required imports")
	}

	if raw, err = format.Source(raw); err != nil {
		return "", errors.Wrap(err, "failed to format source")
	}

	return string(raw), nil
}
