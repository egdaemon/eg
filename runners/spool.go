package runners

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

const defaultPerms = 0700 | os.ModeSetgid

type SpoolDirs struct {
	renamemux   *sync.Mutex
	Downloading string
	Queued      string
	Running     string
	Tombstoned  string
}

func DefaultSpoolDirs() SpoolDirs {
	root := filepath.Join(userx.DefaultCacheDirectory(), "spool")
	dirs := SpoolDirs{
		renamemux:   &sync.Mutex{},
		Downloading: filepath.Join(root, "d"),
		Queued:      filepath.Join(root, "q"),
		Running:     filepath.Join(root, "r"),
		Tombstoned:  filepath.Join(root, "t"),
	}

	errorsx.Log(errors.Wrap(fsx.MkDirs(defaultPerms, dirs.Downloading, dirs.Queued, dirs.Running, dirs.Tombstoned), "unable to make spool directories"))

	return dirs
}

func iddirname(uid uuid.UUID) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(uid.Bytes())
}

func (t SpoolDirs) Download(uid uuid.UUID, name string, content io.Reader) (err error) {
	var (
		dst *os.File
	)

	if err = os.MkdirAll(filepath.Join(t.Downloading, iddirname(uid)), defaultPerms); err != nil {
		return err
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
	return os.Rename(filepath.Join(t.Downloading, iddirname(uid)), filepath.Join(t.Queued, iddirname(uid)))
}

func (t SpoolDirs) Dequeue() (_ string, err error) {
	for {
		dir, err := pop(t.Queued)
		if err != nil {
			return "", err
		}

		if err = t.dequeueRename(dir); fsx.ErrIsNotExist(err) != nil {
			continue
		} else if err != nil {
			return "", err
		}

		return filepath.Join(t.Running, dir.Name()), nil
	}
}

func (t SpoolDirs) dequeueRename(dir fs.DirEntry) error {
	t.renamemux.Lock()
	defer t.renamemux.Unlock()
	return os.Rename(filepath.Join(t.Queued, dir.Name()), filepath.Join(t.Running, dir.Name()))
}

func (t SpoolDirs) Completed(uid uuid.UUID) (err error) {
	// we tombstone before removing. we do this because if there are permission issues within
	// the directory structure deleting as this user running the command will not work and we need to let the OS handle it.
	// We can fail to remove a folder due to  permission issues in files and subfolders. optimistically attempt to remove the
	// the folder. if it succeeds, great. if it fails fall back to moving the work directory into a tombstone directory.
	// this ensures that we can continue processing work. but since we can repeat work multiple times lets give the tombstoned
	// folder a unique time based prefix.
	if err = errorsx.Wrap(os.RemoveAll(filepath.Join(t.Running, iddirname(uid))), "unable to remove work"); err == nil {
		return nil
	} else {
		log.Println(err)
	}

	return errorsx.Wrapf(
		os.Rename(
			filepath.Join(t.Running, iddirname(uid)),
			filepath.Join(t.Tombstoned, fmt.Sprintf("%s-%s", uuid.Must(uuid.NewV7()).String(), iddirname(uid))),
		),
		"unable to tombstone work directory: %s",
		iddirname(uid),
	)
}

func pop(dir string) (popped fs.DirEntry, err error) {
	dirfs, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer dirfs.Close()

	entries, err := dirfs.ReadDir(100)
	if err != nil {
		return nil, err
	}

	return entries[rand.IntN(len(entries))], nil
}
