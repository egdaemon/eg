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
	"github.com/gofrs/uuid/v5"
	"github.com/pkg/errors"
)

// ErrRepoBlocked is returned by Block when another workload for the same
// repo key is already active; the caller's item has been parked and should
// be revisited once Unblock is called for that key.
const ErrRepoBlocked = errorsx.String("repo workload already active")

type SpoolOption func(*SpoolDirs)

func SpoolOptionPopLimit(n int) SpoolOption {
	return func(sd *SpoolDirs) {
		sd.poplimit = n
	}
}

type SpoolDirs struct {
	defaultPerms fs.FileMode
	renamemux    *sync.Mutex
	Downloading  string
	Queued       string
	Running      string
	Tombstoned   string
	Blocked      string
	poplimit     int
}

func NewSpoolDir(root string) SpoolDirs {
	const defaultPerms = 0700 | os.ModeSetgid
	dirs := SpoolDirs{
		defaultPerms: defaultPerms,
		renamemux:    &sync.Mutex{},
		Downloading:  filepath.Join(root, "d"),
		Queued:       filepath.Join(root, "q"),
		Running:      filepath.Join(root, "r"),
		Tombstoned:   filepath.Join(root, "t"),
		Blocked:      filepath.Join(root, "b"),
		poplimit:     100,
	}

	errorsx.Log(errors.Wrap(fsx.MkDirs(defaultPerms, dirs.Downloading, dirs.Queued, dirs.Running, dirs.Tombstoned, dirs.Blocked), "unable to make spool directories"))

	return dirs
}

func DefaultSpoolDirs() SpoolDirs {
	return NewSpoolDir(userx.DefaultCacheDirectory("spool"))
}

func Queued() queued {
	return queued{}
}

type queued struct{}

func (t queued) Dirname(uid uuid.UUID) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(uid.Bytes())
}

func (t queued) Id(dir string) uuid.UUID {
	return uuid.FromBytesOrNil(errorsx.Zero(base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(filepath.Base(dir))))
}

func (t SpoolDirs) Download(uid uuid.UUID, name string, content io.Reader) (err error) {
	var (
		dst *os.File
	)

	if err = os.MkdirAll(filepath.Join(t.Downloading, Queued().Dirname(uid)), t.defaultPerms); err != nil {
		return err
	}

	if dst, err = os.Create(filepath.Join(t.Downloading, Queued().Dirname(uid), name)); err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, content); err != nil {
		return err
	}

	return nil
}

func (t SpoolDirs) Enqueue(uid uuid.UUID) (err error) {
	return os.Rename(filepath.Join(t.Downloading, Queued().Dirname(uid)), filepath.Join(t.Queued, Queued().Dirname(uid)))
}

func (t SpoolDirs) Dequeue() (_ string, err error) {
	for range 100 {
		dir, err := peek(t.Queued, t.poplimit)
		if err != nil {
			return "", err
		}

		if err = t.dequeueRename(dir); fsx.ErrIsNotExist(err) != nil {
			continue
		} else if os.IsExist(err) {
			// when IsExist it means we have the job running. clear from queued.
			errorsx.Log(errorsx.Wrap(os.RemoveAll(filepath.Join(t.Queued, dir.Name())), "failed to clear queued"))
			return "", err
		} else if err != nil {
			return "", err
		}

		return filepath.Join(t.Running, dir.Name()), nil
	}

	return "", errorsx.Wrap(err, "exhausted dequeue attempts, try later")
}

func (t SpoolDirs) dequeueRename(dir fs.DirEntry) error {
	t.renamemux.Lock()
	defer t.renamemux.Unlock()
	return os.Rename(filepath.Join(t.Queued, dir.Name()), filepath.Join(t.Running, dir.Name()))
}

func (t SpoolDirs) Completed(uid uuid.UUID) (err error) {
	// we tombstone before removing. we do this because if there are permission issues within
	// the directory structure deleting as this user running the command will not work and we need to let the OS handle it.
	// We can fail to remove a folder due to a permission issue in files and subfolders. optimistically attempt to remove the
	// the folder. if it succeeds, great. if it fails fall back to moving the work directory into a tombstone directory.
	// this ensures that we can continue processing work. but since we can repeat work multiple times lets give the tombstoned
	// folder a unique time based prefix.
	if err = errorsx.Wrap(os.RemoveAll(filepath.Join(t.Running, Queued().Dirname(uid))), "unable to remove work"); err == nil {
		return nil
	} else {
		log.Println(err)
	}

	return errorsx.Wrapf(
		os.Rename(
			filepath.Join(t.Running, Queued().Dirname(uid)),
			filepath.Join(t.Tombstoned, fmt.Sprintf("%s-%s", uuid.Must(uuid.NewV7()).String(), Queued().Dirname(uid))),
		),
		"unable to tombstone work directory: %s",
		Queued().Dirname(uid),
	)
}

// Block claims the repo key for the caller if it is unclaimed, by creating
// Blocked/<key> and returning nil. If the key is already claimed, dir is
// moved under Blocked/<key> and ErrRepoBlocked is returned so the caller
// knows to back off rather than proceed.
func (t SpoolDirs) Block(key, dir string) (err error) {
	t.renamemux.Lock()
	defer t.renamemux.Unlock()

	blockeddir := filepath.Join(t.Blocked, key)

	if err = os.Rename(dir, filepath.Join(blockeddir, filepath.Base(dir))); err == nil {
		return ErrRepoBlocked
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.Mkdir(blockeddir, t.defaultPerms)
}

// Unblock releases the repo key: any items parked under Blocked/<key> are
// moved back into Queued, then the marker directory itself is removed. Safe
// to call even if the key was never blocked (no-op).
func (t SpoolDirs) Unblock(key string) (err error) {
	t.renamemux.Lock()
	defer t.renamemux.Unlock()

	blockeddir := filepath.Join(t.Blocked, key)

	if err = fs.WalkDir(os.DirFS(blockeddir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		if err := os.Rename(filepath.Join(blockeddir, d.Name()), filepath.Join(t.Queued, d.Name())); err != nil {
			return err
		}

		return fs.SkipDir
	}); fsx.ErrIsNotExist(err) != nil {
		return nil
	} else if err != nil {
		return err
	}

	return os.Remove(blockeddir)
}

func (t SpoolDirs) Discard(dir string) (err error) {
	// we tombstone before removing. we do this because if there are permission issues within
	// the directory structure deleting as this user running the command will not work and we need to let the OS handle it.
	// We can fail to remove a folder due to a permission issue in files and subfolders. optimistically attempt to remove the
	// the folder. if it succeeds, great. if it fails fall back to moving the work directory into a tombstone directory.
	// this ensures that we can continue processing work. but since we can repeat work multiple times lets give the tombstoned
	// folder a unique time based prefix.
	if err = errorsx.Wrap(os.RemoveAll(dir), "unable to remove directory"); err == nil {
		return nil
	} else {
		log.Println(err)
	}

	return errorsx.Wrapf(
		os.Rename(
			dir,
			filepath.Join(t.Tombstoned, fmt.Sprintf("%s-%s", uuid.Must(uuid.NewV7()).String(), filepath.Base(dir))),
		),
		"unable to tombstone work directory: %s",
		Queued().Id(dir),
	)
}

func peek(dir string, n int) (popped fs.DirEntry, err error) {
	dirfs, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer dirfs.Close()

	entries, err := dirfs.ReadDir(n)
	if err != nil {
		return nil, err
	}

	return entries[rand.IntN(len(entries))], nil
}
