package runners

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/internal/envx"
)

type SpoolDirs struct {
	Downloading string
	Queued      string
	Running     string
}

func DefaultSpoolDirs() SpoolDirs {
	root := filepath.Join(envx.String(os.TempDir(), "CACHE_DIRECTORY"), "spool")
	return SpoolDirs{
		Downloading: filepath.Join(root, "downloading"),
		Queued:      filepath.Join(root, "queued"),
		Running:     filepath.Join(root, "running"),
	}
}

func (t SpoolDirs) Download(uid uuid.UUID, name string, content io.Reader) (err error) {
	var (
		dst *os.File
	)

	if err = os.MkdirAll(filepath.Join(t.Downloading, uid.String()), 0700); err != nil {
		return err
	}

	if dst, err = os.Create(filepath.Join(t.Downloading, uid.String(), name)); err != nil {
		return err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, content); err != nil {
		return err
	}

	return nil
}

func (t SpoolDirs) Enqueue(uid uuid.UUID) (err error) {
	if err = os.MkdirAll(t.Queued, 0700); err != nil {
		return err
	}

	return os.Rename(filepath.Join(t.Downloading, uid.String()), filepath.Join(t.Queued, uid.String()))
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
