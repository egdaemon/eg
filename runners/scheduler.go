package runners

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
)

type downloader interface {
	Download(ctx context.Context) error
}

type RemoteDownloader struct {
	ssh.Signer
}

func (t RemoteDownloader) Download(ctx context.Context) (err error) {
	// dirs := DefaultSpoolDirs()
	return NewDownloadClient(nil).Download(ctx)
}

type localdownloader struct{}

func (t localdownloader) Download(ctx context.Context) (err error) {
	var (
		pending *fsnotify.Watcher
	)

	dirs := DefaultSpoolDirs()

	if pending, err = fsnotify.NewWatcher(); err != nil {
		return errors.Wrap(err, "failed to watch queued directory")
	}
	defer func() { errorsx.MaybeLog(errorsx.Wrap(pending.Close(), "failed to close fs watch")) }()

	if err = pending.Add(dirs.Queued); err != nil {
		return errors.Wrap(err, "failed to watch queued directory")
	}

	select {
	case <-pending.Events:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type metadata struct {
	downloader
}

type SchedulerOption func(*metadata)

func SchedulerOptionDownloader(d downloader) SchedulerOption {
	return func(m *metadata) {
		m.downloader = d
	}
}

// runs the scheduler until the context is cancelled.
func Scheduler(ctx context.Context, options ...func(*metadata)) (err error) {
	var (
		s state = staterecover{
			metadata: langx.Clone(
				metadata{
					downloader: localdownloader{},
				},
				options...,
			),
		}
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			s = s.Update(ctx)
		}

		if s == nil {
			return nil
		}
	}
}

type state interface {
	Update(context.Context) state
}

func terminate(cause error) state {
	return stateterminated{
		cause: cause,
	}
}

type stateterminated struct {
	cause error
}

func (t stateterminated) Update(ctx context.Context) state {
	log.Println(errors.Wrap(t.cause, "terminating scheduler due to error"))
	return nil
}

func failure(cause error, n state) state {
	return statefailure{
		cause: cause,
		next:  n,
	}
}

type statefailure struct {
	cause error
	next  state
}

func (t statefailure) Update(ctx context.Context) state {
	log.Println(t.cause)
	return t.next
}

func newdelay(d time.Duration, next state) state {
	return statedelay{
		d:    d,
		next: next,
	}
}

type statedelay struct {
	d    time.Duration
	next state
}

func (t statedelay) Update(ctx context.Context) state {
	select {
	case <-ctx.Done():
		return terminate(ctx.Err())
	case <-time.After(t.d):
		return t.next
	}
}

func idle(md metadata) stateidle {
	return stateidle{
		metadata: md,
	}
}

type stateidle struct {
	metadata
}

func (t stateidle) Update(ctx context.Context) state {
	var (
		err error
	)

	dirs := DefaultSpoolDirs()

	// check if we have work in the queue.
	if dir, err := dirs.Dequeue(); err == nil {
		return beginwork(ctx, t.metadata, dir)
	} else if iox.IgnoreEOF(err) != nil {
		log.Println("unable to dequeue", err)
	}

	// otherwise wait for work.
	if err = os.MkdirAll(dirs.Queued, 0700); err != nil {
		log.Println(errors.Wrap(err, "unable to create queued directory"))
		return newdelay(time.Second, t)
	}

	// check upstream....
	if err = t.metadata.Download(ctx); err == nil {
		return t
	} else if errors.Is(err, context.DeadlineExceeded) {
		return terminate(ctx.Err())
	} else {
		log.Println(err)
		return newdelay(time.Second, t)
	}

	// if pending, err = fsnotify.NewWatcher(); err != nil {
	// 	log.Println(errors.Wrap(err, "failed to watch queued directory"))
	// 	return newdelay(time.Second, t)
	// }
	// defer func() { errorsx.MaybeLog(errorsx.Wrap(pending.Close(), "failed to close fs watch")) }()

	// if err = pending.Add(dirs.Queued); err != nil {
	// 	log.Println(errors.Wrap(err, "failed to watch queued directory"))
	// 	return newdelay(time.Second, t)
	// }

	// select {
	// case <-pending.Events:
	// 	return t
	// case <-ctx.Done():
	// 	return terminate(ctx.Err())
	// }
}

type staterecover struct {
	metadata
}

func (t staterecover) Update(ctx context.Context) state {
	dirs := DefaultSpoolDirs()
	err := fs.WalkDir(os.DirFS(dirs.Running), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		if cause := os.Rename(filepath.Join(dirs.Running, d.Name()), filepath.Join(dirs.Queued, d.Name())); cause != nil {
			return cause
		}

		return fs.SkipDir
	})

	if err != nil {
		return failure(err, idle(t.metadata))
	}

	return idle(t.metadata)
}

func beginwork(ctx context.Context, md metadata, dir string) state {
	var (
		err    error
		ws     workspaces.Context
		tmpdir string
		ragent *Agent
		kernel *os.File
	)

	uid := filepath.Base(dir)
	log.Println("initializing runner", uid, dir)
	m := NewManager(
		ctx,
		langx.Must(filepath.Abs(DefaultManagerDirectory())),
	)

	if tmpdir, err = os.MkdirTemp(envx.String(os.TempDir(), "CACHE_DIRECTORY"), fmt.Sprintf("eg.work.%s.*", uid)); err != nil {
		return failure(err, idle(md))
	}

	if kernel, err = os.Open(filepath.Join(dir, "kernel.tar.gz")); err != nil {
		return failure(err, idle(md))
	}

	if err = tarx.Unpack(filepath.Join(tmpdir, ".eg", ".cache", ".eg"), kernel); err != nil {
		return failure(err, idle(md))
	}

	if ws, err = workspaces.New(ctx, tmpdir, ".eg", "eg"); err != nil {
		return failure(err, idle(md))
	}

	log.Println("workspace", spew.Sdump(ws))

	{
		rootc := filepath.Join(ws.RunnerDir, "Containerfile")

		if err = PrepareRootContainer(rootc); err != nil {
			return failure(err, idle(md))
		}

		cmd := exec.CommandContext(ctx, "podman", "build", "--timestamp", "0", "-t", "eg", "-f", rootc, tmpdir)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		if err = cmd.Run(); err != nil {
			return failure(err, idle(md))
		}
	}

	if ragent, err = m.NewRun(ctx, ws, uid); err != nil {
		return failure(err, idle(md))
	}

	return staterunning{metadata: md, ws: ws, ragent: ragent, dir: dir}
}

type staterunning struct {
	metadata
	ws     workspaces.Context
	ragent *Agent
	dir    string
}

func (t staterunning) Update(ctx context.Context) state {
	log.Println("working", t.dir)

	var (
		err error
		cc  grpc.ClientConnInterface
	)

	m := NewManager(
		ctx,
		langx.Must(filepath.Abs(DefaultManagerDirectory())),
	)

	if cc, err = m.Dial(ctx, t.ragent.id); err != nil {
		return failure(err, idle(t.metadata))
	}

	runner := c8s.NewProxyClient(cc)

	select {
	case <-ctx.Done():
		return terminate(ctx.Err())
	default:
		_, err := runner.Module(ctx, &c8s.ModuleRequest{
			Image: "eg",
			Name:  fmt.Sprintf("eg-%s", t.ragent.id),
			Mdir:  t.ws.ModuleDir,
			Options: []string{
				"--env", "EG_BIN",
				"--volume", fmt.Sprintf("%s:/opt/egmodule.wasm:ro", filepath.Join(t.ws.RunnerDir, "main.wasm")),
				"--volume", fmt.Sprintf("%s:/opt/eg:O", t.ws.Root),
			},
		})
		if err != nil {
			return failure(err, idle(t.metadata))
		}

		return idle(t.metadata)
	}
}

//go:embed DefaultContainerfile
var embedded embed.FS

func PrepareRootContainer(cpath string) (err error) {
	var (
		c   fs.File
		dst *os.File
	)

	log.Println("default container path", cpath)
	if c, err = embedded.Open("DefaultContainerfile"); err != nil {
		return err
	}
	defer c.Close()

	if dst, err = os.OpenFile(cpath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600); err != nil {
		return err
	}

	if _, err = io.Copy(dst, c); err != nil {
		return err
	}

	return nil
}
