package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"github.com/fsnotify/fsnotify"
	"github.com/gofrs/uuid"
)

type downloader interface {
	Download(ctx context.Context) error
}

type completion interface {
	Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader, analytics io.Reader) (err error)
}

type noopcompletion struct{}

func (t noopcompletion) Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader, analytics io.Reader) (err error) {
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
	logVerbosity int
	reload       chan error
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

func QueueOptionLogVerbosity(n int) QueueOption {
	return func(m *metadata) {
		m.logVerbosity = n
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
	if err = eg.PrepareRootContainer(path); err != nil {
		return errorsx.Wrapf(err, "preparing root container failed: %s", path)
	}

	cmd := exec.CommandContext(ctx, "podman", "build", "-t", "eg", "-f", path, dir)
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

// func LoadQueue(ctx context.Context, options ...func(*metadata)) (err error) {
// 	// monitor for reload signals, can't use the context because we
// 	// dont want to interrupt running work but only want to stop after a run.
// 	reload := make(chan error, 1)
// 	go debugx.OnSignal(func() error {
// 		reload <- errorsx.String("reload daemon signal received")
// 		close(reload)
// 		return nil
// 	})(ctx, syscall.SIGHUP)

// 	if err = BuildRootContainer(ctx); err != nil {
// 		return err
// 	}

// 	for {
// 		current, err := determineload(ctx)
// 		if err != nil {
// 			log.Println("unable to determine load deferring", err)
// 		}

// 		log.Println("queue2", spew.Sdump(current))
// 	}
// }

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
		err      error
		ws       workspaces.Context
		tmpdir   string
		ragent   *Agent
		archive  *os.File
		_uid     uuid.UUID
		encoded  []byte
		metadata = &EnqueuedDequeueResponse{
			Enqueued: &Enqueued{Entry: "main.wasm"},
		}
	)

	if _uid, err = uidfrompath(filepath.Join(dir, "uuid")); err != nil {
		return failure(err, idle(md))
	}
	uid := _uid.String()

	log.Println("initializing runner", uid, dir)
	tmpdir = filepath.Join(dir, "work")

	if err = fsx.MkDirs(0770, tmpdir); err != nil {
		return failure(err, idle(md))
	}

	if encoded, err = os.ReadFile(filepath.Join(dir, "metadata.json")); err != nil {
		return failure(err, idle(md))
	}

	if err = json.Unmarshal(encoded, metadata); err != nil {
		return failure(err, idle(md))
	}

	debugx.Println("metadata", spew.Sdump(metadata.Enqueued))

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

	if ws, err = workspaces.New(ctx, tmpdir, eg.DefaultModuleDirectory(), "eg"); err != nil {
		return discard(uid, md, failure(errorsx.Wrap(err, "unable to setup workspace"), idle(md)))
	}

	debugx.Println("workspace", spew.Sdump(ws))

	if err = tarx.Unpack(filepath.Join(ws.Root, ws.RuntimeDir), archive); err != nil {
		return completed(metadata.Enqueued, md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "unable to unpack archive"))
	}

	if err = wasix.WarmCacheDirectory(ctx, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir))); err != nil {
		log.Println("unable to prewarm wasi cache", err)
	} else {
		log.Println("wasi cache prewarmed", wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir)))
	}

	{
		rootc := filepath.Join(filepath.Join(ws.Root, ws.RuntimeDir), "Containerfile")

		if err = eg.PrepareRootContainer(rootc); err != nil {
			return completed(metadata.Enqueued, md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "preparing root container failed"))
		}

		cmd := exec.CommandContext(ctx, "podman", "build", "-t", "eg", "-f", rootc, tmpdir)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		if err = cmd.Run(); err != nil {
			return completed(metadata.Enqueued, md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "build failed"))
		}
	}

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")

	envb := envx.Build().FromPath(environpath).
		Var(gitx.EnvAuthEGAccessToken, metadata.AccessToken).
		Var(eg.EnvComputeRunID, uid).
		Var(eg.EnvComputeAccountID, metadata.Enqueued.AccountId).
		Var(eg.EnvComputeVCS, metadata.Enqueued.VcsUri).
		Var(eg.EnvComputeTTL, time.Duration(metadata.Enqueued.Ttl).String()).
		Var(eg.EnvComputeLoggingVerbosity, envx.String(strconv.Itoa(md.logVerbosity), eg.EnvComputeLoggingVerbosity))

	// envx.Debug(errorsx.Zero(envb.Environ())...)

	if err = envb.WriteTo(environpath); err != nil {
		return completed(metadata.Enqueued, md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "failed to update environment file"))
	}

	aopts := make([]AgentOption, 0, len(md.agentopts)+32)
	aopts = append(aopts, md.agentopts...)
	aopts = append(
		aopts,
		AgentOptionEGBin(errorsx.Zero(exec.LookPath(os.Args[0]))),
		AgentOptionEnviron(environpath),
		AgentOptionCommandLine("--env-file", environpath),
		AgentOptionCores(metadata.Enqueued.Cores),
		AgentOptionMemory(metadata.Enqueued.Memory),
		AgentOptionCommandLine("--userns", "host"),       // properly map host user into containers.
		AgentOptionCommandLine("--cap-add", "NET_ADMIN"), // required for loopback device creation inside the container
		AgentOptionCommandLine("--cap-add", "SYS_ADMIN"), // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		AgentOptionCommandLine("--device", "/dev/fuse"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		AgentOptionCommandLine("--pids-limit", "-1"),     // more bullshit. without this we get "Error: OCI runtime error: crun: the requested cgroup controller `pids` is not available"
	)

	m := NewManager(
		ctx,
	)

	if ragent, err = m.NewRun(ctx, ws, uid, aopts...); err != nil {
		return completed(metadata.Enqueued, md, tmpdir, ws, uid, 0, errorsx.Wrap(err, "run failure"))
	}

	return staterunning{metadata: md, workload: metadata.Enqueued, ws: ws, ragent: ragent, dir: dir, tmpdir: tmpdir, entry: metadata.Enqueued.Entry}
}

func cacheprefix(enq *Enqueued) string {
	if prefix, _, ok := strings.Cut(enq.AccountId, "-"); ok {
		return prefix
	}

	return enq.AccountId
}

func cachebucket(enq *Enqueued) string {
	return md5x.String(enq.AccountId + enq.VcsUri)
}

type staterunning struct {
	metadata
	workload *Enqueued
	ws       workspaces.Context
	ragent   *Agent
	dir      string
	tmpdir   string
	entry    string
}

func (t staterunning) Update(ctx context.Context) state {
	log.Println("work initiated", t.dir)
	select {
	case <-ctx.Done():
		return terminate(ctx.Err())
	default:
		var (
			err          error
			logdst       *os.File
			containerdir = userx.DefaultCacheDirectory("wcache", cacheprefix(t.workload), cachebucket(t.workload), "containers")
			cachedir     = userx.DefaultCacheDirectory("wcache", cacheprefix(t.workload), cachebucket(t.workload), "workloadcache")
			logpath      = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "daemon.log")
		)

		if err = fsx.MkDirs(0770, containerdir, cachedir); err != nil {
			return terminate(errorsx.Wrap(err, "unable to setup container and cache directories"))
		}

		log.Println("workload root cachedir", cachedir)

		if logdst, err = os.Create(logpath); err != nil {
			return completed(t.workload, t.metadata, t.tmpdir, t.ws, t.ragent.id, 0, err)
		}

		defer logdst.Close()
		options := append(
			t.ragent.Options(),
			"--volume", AgentMountReadWrite(containerdir, "/var/lib/containers"),
			"--volume", AgentMountReadOnly(filepath.Join(t.ws.Root, t.ws.RuntimeDir, t.entry), eg.DefaultMountRoot(eg.ModuleBin)),
			"--volume", AgentMountReadWrite(filepath.Join(t.ws.Root, t.ws.RuntimeDir), eg.DefaultMountRoot(eg.RuntimeDirectory)),
			"--volume", AgentMountReadWrite(filepath.Join(t.ws.Root, t.ws.WorkingDir), eg.DefaultMountRoot(eg.WorkingDirectory)),
			"--volume", AgentMountReadWrite(filepath.Join(t.ws.Root, t.ws.TemporaryDir), eg.DefaultMountRoot(eg.TempDirectory)),
			"--volume", AgentMountReadWrite(cachedir, eg.DefaultMountRoot(eg.CacheDirectory)),
		)

		logger := log.New(io.MultiWriter(os.Stderr, logdst), t.ragent.id, log.Flags())
		prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = t.ws.Root
			cmd.Stdout = logger.Writer()
			cmd.Stderr = logger.Writer()
			return cmd
		}

		ts := time.Now()
		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		err = c8s.PodmanModule(ctx, prepcmd, "eg", fmt.Sprintf("eg-%s", t.ragent.id), t.ws.RuntimeDir, options...)
		return completed(t.workload, t.metadata, t.tmpdir, t.ws, t.ragent.id, time.Since(ts), err)
	}
}

func completed(workload *Enqueued, md metadata, tmpdir string, ws workspaces.Context, id string, duration time.Duration, cause error) statecompleted {
	return statecompleted{
		workload: workload,
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
	workload *Enqueued
	ws       workspaces.Context
	tmpdir   string
	id       string
	cause    error
	duration time.Duration
}

func (t statecompleted) Update(ctx context.Context) state {
	var (
		logpath       = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "daemon.log")
		analyticspath = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "analytics.db")
	)

	dirs := DefaultSpoolDirs()
	log.Println("completed", t.workload.AccountId, t.workload.VcsUri, filepath.Join(dirs.Running, t.id), t.cause)

	// fsx.PrintDir(os.DirFS(filepath.Join(t.ws.Root, t.ws.RuntimeDir)))

	logs, err := os.Open(logpath)
	if err != nil {
		return discard(t.id, t.metadata, failure(errorsx.Wrap(err, "unable open logs for upload"), idle(t.metadata)))
	}
	defer logs.Close()

	analytics, err := os.Open(analyticspath)
	if err != nil {
		return discard(t.id, t.metadata, failure(errorsx.Wrap(err, "unable open analytics for upload"), idle(t.metadata)))
	}
	defer analytics.Close()

	if err = t.metadata.completion.Upload(ctx, t.id, t.duration, t.cause, logs, analytics); err != nil {
		return discard(t.id, t.metadata, failure(errorsx.Wrapf(err, "unable to upload completion: %s", t.id), idle(t.metadata)))
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
