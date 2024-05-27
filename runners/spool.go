package runners

import (
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

type SpoolDirs struct {
	Downloading string
	Queued      string
	Running     string
}

func DefaultSpoolDirs() SpoolDirs {
	root := filepath.Join(userx.DefaultCacheDirectory(), "spool")
	dirs := SpoolDirs{
		Downloading: filepath.Join(root, "d"),
		Queued:      filepath.Join(root, "q"),
		Running:     filepath.Join(root, "r"),
	}

	errorsx.Log(errors.Wrap(fsx.MkDirs(0700, dirs.Downloading, dirs.Queued, dirs.Running), "unable to make spool directories"))

	return dirs
}

func iddirname(uid uuid.UUID) string {
	return hex.EncodeToString(uid.Bytes()[:6])
}

func uidfrompath(path string) (uid uuid.UUID, err error) {
	var (
		encodedid []byte
	)
	if encodedid, err = os.ReadFile(path); err != nil {
		return uid, errorsx.Wrapf(err, "unable read uuid from disk: %s", path)
	}

	uid, err = uuid.FromBytes(encodedid)
	return uid, errorsx.Wrapf(err, "unable read uuid from disk: %s", path)
}

func (t SpoolDirs) Download(uid uuid.UUID, name string, content io.Reader) (err error) {
	var (
		dst *os.File
	)

	if err = os.MkdirAll(filepath.Join(t.Downloading, iddirname(uid)), 0700); err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(t.Downloading, iddirname(uid), "uuid"), uid.Bytes(), 0600); err != nil {
		return errorsx.Wrap(err, "unable to write run id to disk")
	}

	if dst, err = os.Create(filepath.Join(t.Downloading, iddirname(uid), name)); err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, content); err != nil {
		return err
	}

	return nil
}

func (t SpoolDirs) Enqueue(uid uuid.UUID) (err error) {
	if err = os.MkdirAll(t.Queued, 0700); err != nil {
		return err
	}

	return os.Rename(filepath.Join(t.Downloading, iddirname(uid)), filepath.Join(t.Queued, iddirname(uid)))
}

func (t SpoolDirs) Dequeue() (_ string, err error) {
	if err = os.MkdirAll(t.Running, 0700); err != nil {
		return "", err
	}

	dir, err := pop(t.Queued)
	if err != nil {
		return "", err
	}

	return filepath.Join(t.Running, dir.Name()), os.Rename(filepath.Join(t.Queued, dir.Name()), filepath.Join(t.Running, dir.Name()))
}

func (t SpoolDirs) Completed(uid uuid.UUID) (err error) {
	return errorsx.Wrap(os.RemoveAll(filepath.Join(t.Running, iddirname(uid))), "unable to remove work")
}

func pop(dir string) (popped fs.DirEntry, err error) {
	dirfs, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer dirfs.Close()

	entries, err := dirfs.ReadDir(1)
	if err != nil {
		return nil, err
	}

	return entries[0], nil
}
