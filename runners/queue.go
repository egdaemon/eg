package runners

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"github.com/fsnotify/fsnotify"
	"github.com/gofrs/uuid"
	"google.golang.org/grpc"
)

type downloader interface {
	Download(ctx context.Context) error
}

type completion interface {
	Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader) (err error)
}

type noopcompletion struct{}

func (t noopcompletion) Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader) (err error) {
	log.Println("noop completion is being used", id)
	return nil
}

type localdownloader struct{}

func (t localdownloader) Download(ctx context.Context) (err error) {
	var (
		pending *fsnotify.Watcher
	)

	dirs := DefaultSpoolDirs()

	if pending, err = fsnotify.NewWatcher(); err != nil {
		return errorsx.Wrap(err, "failed to watch queued directory")
	}
	defer func() { errorsx.Log(errorsx.Wrap(pending.Close(), "failed to close fs watch")) }()

	if err = pending.Add(dirs.Queued); err != nil {
		return errorsx.Wrap(err, "failed to watch queued directory")
	}

	select {
	case <-pending.Events:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type metadata struct {
	reload chan error
	downloader
	completion
	agentopts []AgentOption
}

type QueueOption func(*metadata)

func QueueOptionNoop(m *metadata) {}
func QueueOptionAgentOptions(options ...AgentOption) QueueOption {
	return func(m *metadata) {
		m.agentopts = options
	}
}

func QueueOptionCompletion(c completion) QueueOption {
	return func(m *metadata) {
		m.completion = c
	}
}

func BuildRootContainer(ctx context.Context) error {
	tmpdir, err := os.MkdirTemp("", "eg.container.build")
	if err != nil {
		return errorsx.Wrap(err, "unable to preprate root container")
	}
	defer os.RemoveAll(tmpdir)
	rootc := filepath.Join(tmpdir, "Containerfile")

	return BuildRootContainerPath(ctx, tmpdir, rootc)
}

func BuildRootContainerPath(ctx context.Context, dir, path string) (err error) {
	if err = PrepareRootContainer(path); err != nil {
		return errorsx.Wrapf(err, "preparing root container failed: %s", path)
	}

	cmd := exec.CommandContext(ctx, "podman", "build", "--timestamp", "0", "-t", "eg", "-f", path, dir)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err = cmd.Run(); err != nil {
		return errorsx.Wrap(err, "build failed")
	}

	return nil
}

// runs the scheduler until the context is cancelled.
func Queue(ctx context.Context, options ...func(*metadata)) (err error) {
	// monitor for reload signals, can't use the context because we
	// dont want to interrupt running work but only want to stop after a run.
	reload := make(chan error, 1)
	go debugx.OnSignal(func() error {
		reload <- errorsx.String("reload daemon signal received")
		close(reload)
		return nil
	})(ctx, syscall.SIGHUP)

	var (
		s state = staterecover{
			metadata: langx.Clone(
				metadata{
					reload:     reload,
					completion: noopcompletion{},
					downloader: localdownloader{},
				},
				options...,
			),
		}
	)

	if err = BuildRootContainer(ctx); err != nil {
		return err
	}

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
	log.Println(errorsx.Wrap(t.cause, "terminating scheduler due to error"))
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

	select {
	case <-ctx.Done():
		return failure(ctx.Err(), nil)
	case cause := <-t.metadata.reload:
		return failure(cause, nil)
	default:
	}

	// check if we have work in the queue.
	if dir, err := dirs.Dequeue(); err == nil {
		return beginwork(ctx, t.metadata, dir)
	} else if iox.IgnoreEOF(err) != nil {
		log.Println("unable to dequeue", err)
	}

	// otherwise wait for work.
	if err = os.MkdirAll(dirs.Queued, 0700); err != nil {
		log.Println(errorsx.Wrap(err, "unable to create queued directory"))
		return newdelay(time.Second, t)
	}

	// check upstream....
	if err = t.metadata.Download(ctx); errors.Is(err, context.DeadlineExceeded) {
		return terminate(err)
	} else if err != nil {
		log.Println(err)
		return newdelay(time.Second, t)
	}

	return t
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
		return failure(errorsx.Wrap(err, "recovery failed"), idle(t.metadata))
	}

	return idle(t.metadata)
}

func beginwork(ctx context.Context, md metadata, dir string) state {
	var (
		err     error
		ws      workspaces.Context
		tmpdir  string
		ragent  *Agent
		archive *os.File
		_uid    uuid.UUID
	)

	if _uid, err = uidfrompath(filepath.Join(dir, "uuid")); err != nil {
		return failure(err, idle(md))
	}
	uid := _uid.String()

	log.Println("initializing runner", uid, dir)

	tmpdir = filepath.Join(dir, "work")
	if err = os.MkdirAll(tmpdir, 0700); err != nil {
		return failure(err, idle(md))
	}

	defer func() {
		if err == nil {
			return
		}

		log.Println("error detected clearing tmp directory", tmpdir)
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove tmpdir"))
	}()

	if archive, err = os.Open(filepath.Join(dir, "archive.tar.gz")); err != nil {
		return discard(uid, md, failure(errorsx.Wrap(err, "unable to read archive"), idle(md)))
	}

	errorsx.Log(tarx.Inspect(archive))

	if ws, err = workspaces.New(ctx, tmpdir, ".eg", "eg"); err != nil {
		return discard(uid, md, failure(errorsx.Wrap(err, "unable to setup workspace"), idle(md)))
	}

	log.Println("workspace", spew.Sdump(ws))

	if err = tarx.Unpack(filepath.Join(ws.Root, ws.RuntimeDir), archive); err != nil {
		return completed(md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "unable to unpack archive"))
	}

	if err = wasix.WarmCacheDirectory(ctx, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir))); err != nil {
		log.Println("unable to prewarm wasi cache", err)
	} else {
		log.Println("wasi cache prewarmed", wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir)))
	}

	{
		rootc := filepath.Join(filepath.Join(ws.Root, ws.RuntimeDir), "Containerfile")

		if err = PrepareRootContainer(rootc); err != nil {
			return completed(md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "preparing root container failed"))
		}

		cmd := exec.CommandContext(ctx, "podman", "build", "--timestamp", "0", "-t", "eg", "-f", rootc, tmpdir)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		if err = cmd.Run(); err != nil {
			return completed(md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "build failed"))
		}
	}

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")
	// environ := errorsx.Zero(envx.FromPath(environpath))
	// envx.Debug(environ...)

	aopts := make([]AgentOption, 0, len(md.agentopts)+2)
	aopts = append(aopts, md.agentopts...)
	aopts = append(
		aopts,
		AgentOptionEGBin(errorsx.Zero(exec.LookPath(os.Args[0]))),
		AgentOptionEnviron(environpath),
		// AgentOptionEnv("EG_CACHE_DIRECTORY", filepath.Join(ws.Root, ws.ModuleDir, ws.CacheDir)),
	)

	m := NewManager(
		ctx,
	)

	if ragent, err = m.NewRun(ctx, ws, uid, aopts...); err != nil {
		return completed(md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "run failure"))
	}

	return staterunning{metadata: md, ws: ws, ragent: ragent, dir: dir, tmpdir: tmpdir}
}

type staterunning struct {
	metadata
	ws     workspaces.Context
	ragent *Agent
	dir    string
	tmpdir string
}

func (t staterunning) Update(ctx context.Context) state {
	log.Println("work initiated", t.dir)

	var (
		err error
		cc  grpc.ClientConnInterface
	)

	m := NewManager(
		ctx,
	)

	if cc, err = m.Dial(ctx, t.ws); err != nil {
		return failure(err, idle(t.metadata))
	}
	runner := c8s.NewProxyClient(cc)

	select {
	case <-ctx.Done():
		return terminate(ctx.Err())
	default:
		ts := time.Now()
		_, err := runner.Module(ctx, &c8s.ModuleRequest{
			Image: "eg",
			Name:  fmt.Sprintf("eg-%s", t.ragent.id),
			Mdir:  t.ws.RuntimeDir,
			Options: []string{
				"--env", "EG_BIN",
				"--volume", fmt.Sprintf("%s:/opt/egmodule.wasm:ro", filepath.Join(t.ws.Root, t.ws.RuntimeDir, "main.wasm")),
			},
		})

		return completed(t.metadata, t.tmpdir, t.ws, t.ragent.id, time.Since(ts), errorsx.Compact(err, t.ragent.Close()))
	}
}

func completed(md metadata, tmpdir string, ws workspaces.Context, id string, duration time.Duration, cause error) statecompleted {
	return statecompleted{
		ws:       ws,
		metadata: md,
		tmpdir:   tmpdir,
		id:       id,
		cause:    cause,
		duration: duration,
	}
}

type statecompleted struct {
	metadata
	ws       workspaces.Context
	tmpdir   string
	id       string
	cause    error
	duration time.Duration
}

func (t statecompleted) Update(ctx context.Context) state {
	var (
		logpath = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "daemon.log")
	)

	dirs := DefaultSpoolDirs()
	log.Println("completed", t.id, filepath.Join(dirs.Running, t.id), t.cause)

	// fsx.PrintDir(os.DirFS(filepath.Join(t.ws.Root, t.ws.RuntimeDir)))

	logs, err := os.Open(logpath)
	if err != nil {
		return discard(t.id, t.metadata, failure(errorsx.Wrap(err, "unable open logs for upload"), idle(t.metadata)))
	}
	defer logs.Close()

	if err = t.metadata.completion.Upload(ctx, t.id, t.duration, t.cause, logs); err != nil {
		return discard(t.id, t.metadata, failure(errorsx.Wrap(err, "unable upload completion"), idle(t.metadata)))
	}

	if t.cause != nil {
		return discard(t.id, t.metadata, failure(errorsx.Wrap(t.cause, "work failed"), idle(t.metadata)))
	}

	return discard(t.id, t.metadata, idle(t.metadata))
}

func discard(id string, md metadata, next state) statediscard {
	return statediscard{
		id:       id,
		metadata: md,
		n:        next,
	}
}

type statediscard struct {
	metadata
	id string
	n  state
}

func (t statediscard) Update(ctx context.Context) state {
	dirs := DefaultSpoolDirs()
	if err := dirs.Completed(uuid.FromStringOrNil(t.id)); err != nil {
		return failure(errorsx.Wrap(err, "completion failed"), t.n)
	}

	return t.n
}

//go:embed DefaultContainerfile
var embedded embed.FS

func PrepareRootContainer(cpath string) (err error) {
	var (
		c   fs.File
		dst *os.File
	)

	// log.Println("---------------------- Prepare Root Container Initiated ----------------------")
	// defer log.Println("---------------------- Prepare Root Container Completed ----------------------")

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
