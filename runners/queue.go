package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/workspaces"
	"github.com/fsnotify/fsnotify"
	"github.com/gofrs/uuid"

	"github.com/alitto/pond/v2"
)

const (
	workdirname = "work"
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

	for {
		select {
		case evt := <-pending.Events:
			if evt.Op == fsnotify.Create {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type metadata struct {
	id           int
	logVerbosity int
	reload       chan error
	downloader
	completion
	dirs      *SpoolDirs
	rm        *ResourceManager
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
	debugx.Println("building root container initiated")
	defer debugx.Println("building root container completed")

	if err = eg.PrepareRootContainer(path); err != nil {
		return errorsx.Wrapf(err, "preparing root container failed: %s", path)
	}

	cmd := exec.CommandContext(ctx, "podman", "build", "-t", "eg", "-f", path, dir)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err = execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrap(err, "build failed")
	}

	return nil
}

func workload(ctx context.Context, id int, delay time.Duration, rm *ResourceManager, dirs *SpoolDirs, reload chan error, options ...func(*metadata)) error {
	var (
		s state = newdelay(
			delay,
			idle(langx.Clone(
				metadata{
					id:         id,
					rm:         rm,
					reload:     reload,
					completion: noopcompletion{},
					downloader: localdownloader{},
					dirs:       dirs,
				},
				options...,
			)),
		)
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			debugx.Printf("running %d - %T\n", id, s)
			s = s.Update(ctx)
		}

		if s == nil {
			return nil
		}
	}
}

func workloadcapacity() int {
	return envx.Int(runtime.NumCPU(), eg.EnvComputeWorkloadCapacity)
}

// runs the scheduler until the context is cancelled.
func Queue(ctx context.Context, rm *ResourceManager, options ...func(*metadata)) (err error) {
	// monitor for reload signals, can't use the context because we
	// dont want to interrupt running work but only want to stop after a run.
	reload := make(chan error, 1)
	go debugx.OnSignal(func() error {
		reload <- errorsx.String("reload daemon signal received")
		close(reload)
		return nil
	})(ctx, syscall.SIGHUP)

	dirs := DefaultSpoolDirs()
	var (
		md = langx.Clone(
			metadata{
				id:         0,
				rm:         rm,
				reload:     reload,
				completion: noopcompletion{},
				downloader: localdownloader{},
				dirs:       &dirs,
			},
			options...,
		)
	)

	if cause := recover(ctx, md); cause != nil {
		log.Println("recovery failed, continuing", cause)
	}

	pool := pond.NewPool(workloadcapacity())
	workers := make([]pond.Task, 0, pool.MaxConcurrency())

	for i := 0; i < pool.MaxConcurrency(); i++ {
		// we defer startup of workloads to avoid thundering herd and synchronization
		delay := backoff.DynamicHashDuration(time.Second, strconv.FormatInt(int64(i), 36))
		log.Println("workload", i, "deferred", delay)
		workers = append(workers, pool.SubmitErr(func() error {
			return workload(ctx, i, delay, rm, &dirs, reload, options...)
		}))
	}

	pool.StopAndWait()

	for _, t := range workers {
		if err := t.Wait(); err != nil {
			return err
		}
	}

	return nil
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
	select {
	case <-ctx.Done():
		return failure(ctx.Err(), nil)
	case cause := <-t.metadata.reload:
		return failure(cause, nil)
	default:
	}

	// check if we have work in the queue.
	if dir, err := t.metadata.dirs.Dequeue(); err == nil {
		return beginwork(ctx, t.metadata, dir)
	} else if iox.IgnoreEOF(err) != nil {
		log.Println("unable to dequeue", err)
	}

	// check the spool directory....
	if err := t.metadata.Download(ctx); errors.Is(err, context.DeadlineExceeded) {
		return terminate(err)
	} else if errors.Is(err, context.Canceled) {
		return nil
	} else if err != nil {
		return failure(err, t)
	}

	return t
}

func recover(_ context.Context, md metadata) error {
	return fs.WalkDir(os.DirFS(md.dirs.Running), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		// reset the working directory prior to restarting.
		if err = os.RemoveAll(filepath.Join(md.dirs.Running, d.Name(), workdirname)); err != nil {
			return err
		}

		if cause := os.Rename(filepath.Join(md.dirs.Running, d.Name()), filepath.Join(md.dirs.Queued, d.Name())); fsx.ErrIsNotExist(cause) != nil {
			return fs.SkipDir
		} else if cause != nil {
			return cause
		}

		return fs.SkipDir
	})
}

func beginwork(ctx context.Context, md metadata, dir string) state {
	var (
		err      error
		ws       workspaces.Context
		ragent   *Agent
		archive  *os.File
		encoded  []byte
		workload = &EnqueuedDequeueResponse{
			Enqueued: &Enqueued{Entry: "main.wasm"},
		}
	)

	log.Println("initializing runner", dir)

	if encoded, err = os.ReadFile(filepath.Join(dir, "metadata.json")); err != nil {
		return failure(err, idle(md))
	}

	if err = json.Unmarshal(encoded, workload); err != nil {
		return failure(err, idle(md))
	}

	log.Println("metadata", workload.Enqueued.Id)

	md.rm.Reserve(NewRuntimeResourcesFromDequeued(workload.Enqueued))

	if archive, err = os.Open(filepath.Join(dir, "archive.tar.gz")); err != nil {
		return completed(workload.Enqueued, md, ws, 0, errorsx.Wrap(err, "unable to read archive"))
	}

	errorsx.Log(tarx.Inspect(archive))

	if ws, err = workspaces.New(ctx, filepath.Join(dir, workdirname), eg.DefaultModuleDirectory(), eg.WorkingDirectory); err != nil {
		return completed(workload.Enqueued, md, ws, 0, errorsx.Wrap(err, "unable to setup workspace"))
	}

	// debugx.Println("workspace", spew.Sdump(ws))

	if err = tarx.Unpack(filepath.Join(ws.Root, ws.RuntimeDir), archive); err != nil {
		return completed(workload.Enqueued, md, ws, 0, errorsx.Wrap(err, "unable to unpack archive"))
	}

	if err = wasix.WarmCacheDirectory(ctx, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.CacheDir, eg.DefaultModuleDirectory()))); err != nil {
		log.Println("unable to prewarm wasi cache", err)
	} else {
		log.Println("wasi cache prewarmed", wasix.WazCacheDir(filepath.Join(ws.Root, ws.CacheDir, eg.DefaultModuleDirectory())))
	}

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")

	envb := envx.Build().FromPath(environpath).
		Var(gitx.EnvAuthEGAccessToken, workload.AccessToken).
		Var(eg.EnvComputeRunID, workload.Enqueued.Id).
		Var(eg.EnvComputeAccountID, workload.Enqueued.AccountId).
		Var(eg.EnvComputeVCS, workload.Enqueued.VcsUri).
		Var(eg.EnvComputeTTL, time.Duration(workload.Enqueued.Ttl).String()).
		Var(eg.EnvComputeLoggingVerbosity, envx.String(strconv.Itoa(md.logVerbosity), eg.EnvComputeLoggingVerbosity))

	// envx.Debug(errorsx.Zero(envb.Environ())...)

	if err = envb.WriteTo(environpath); err != nil {
		return completed(workload.Enqueued, md, ws, 0, errorsx.Wrap(err, "failed to update environment file"))
	}

	aopts := make([]AgentOption, 0, len(md.agentopts)+32)
	aopts = append(aopts, md.agentopts...)
	aopts = append(
		aopts,
		AgentOptionEGBin(errorsx.Zero(exec.LookPath(os.Args[0]))),
		AgentOptionEnviron(environpath),
		AgentOptionCommandLine("--env-file", environpath),
		AgentOptionCores(workload.Enqueued.Cores),
		AgentOptionMemory(workload.Enqueued.Memory),
		AgentOptionHostOS(),
		AgentOptionCommandLine("--pids-limit", "-1"), // more bullshit. without this we get "Error: OCI runtime error: crun: the requested cgroup controller `pids` is not available"
	)

	m := NewManager(
		ctx,
	)

	if ragent, err = m.NewRun(ctx, ws, workload.Enqueued.Id, aopts...); err != nil {
		return completed(workload.Enqueued, md, ws, 0, errorsx.Wrap(err, "run failure"))
	}

	return staterunning{metadata: md, workload: workload.Enqueued, ws: ws, ragent: ragent, dir: dir}
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
}

func (t staterunning) Update(ctx context.Context) state {
	select {
	case <-ctx.Done():
		return terminate(ctx.Err())
	default:
	}

	var (
		err           error
		logdst        *os.File
		containerdir  = userx.DefaultCacheDirectory("wcache", cacheprefix(t.workload), cachebucket(t.workload), "containers")
		cachedir      = userx.DefaultCacheDirectory("wcache", cacheprefix(t.workload), cachebucket(t.workload), "workloadcache")
		logpath       = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "daemon.log")
		analyticspath = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "analytics.db")
	)

	if err = fsx.MkDirs(0770, containerdir, cachedir); err != nil {
		return completed(t.workload, t.metadata, t.ws, 0, errorsx.Wrap(err, "unable to setup container and cache directories"))
	}

	log.Println("workload root cachedir", cachedir)

	if logdst, err = os.Create(logpath); err != nil {
		return completed(t.workload, t.metadata, t.ws, 0, err)
	}
	defer logdst.Close()

	if err = events.InitializeDB(ctx, analyticspath); err != nil {
		return completed(t.workload, t.metadata, t.ws, 0, err)
	}

	options := append(
		t.ragent.Options(),
		"--replace", // during recovery we might have a container already running.
		"--volume", AgentMountReadWrite(containerdir, "/var/lib/containers"),
		"--volume", AgentMountReadOnly(filepath.Join(t.ws.Root, t.ws.RuntimeDir, t.workload.Entry), eg.DefaultMountRoot(eg.ModuleBin)),
		"--volume", AgentMountReadWrite(filepath.Join(t.ws.Root, t.ws.RuntimeDir), eg.DefaultMountRoot(eg.RuntimeDirectory)),
		"--volume", AgentMountReadWrite(filepath.Join(t.ws.Root, t.ws.WorkingDir), eg.DefaultMountRoot(eg.WorkingDirectory)),
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
	err = c8sproxy.PodmanModule(ctx, prepcmd, "eg", fmt.Sprintf("eg-%s", t.ragent.id), t.ws.RuntimeDir, options...)
	return completed(t.workload, t.metadata, t.ws, time.Since(ts), err)
}

func completed(workload *Enqueued, md metadata, ws workspaces.Context, duration time.Duration, cause error) statecompleted {
	return statecompleted{
		workload: workload,
		ws:       ws,
		metadata: md,
		cause:    cause,
		duration: duration,
	}
}

type statecompleted struct {
	metadata
	workload *Enqueued
	ws       workspaces.Context
	cause    error
	duration time.Duration
}

func (t statecompleted) Update(ctx context.Context) state {
	var (
		logpath       = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "daemon.log")
		analyticspath = filepath.Join(t.ws.Root, t.ws.RuntimeDir, "analytics.db")
	)

	log.Println("completed", t.workload.Id, t.workload.AccountId, t.workload.VcsUri, t.ws.Root, t.duration, t.cause)

	logs, err := os.Open(logpath)
	if err != nil {
		return discard(t.workload, t.metadata, failure(errorsx.Wrap(err, "unable open logs for upload"), idle(t.metadata)))
	}
	defer logs.Close()

	analytics, err := os.Open(analyticspath)
	if err != nil {
		return discard(t.workload, t.metadata, failure(errorsx.Wrap(err, "unable open analytics for upload"), idle(t.metadata)))
	}
	defer analytics.Close()
	if err = t.metadata.completion.Upload(ctx, t.workload.Id, t.duration, t.cause, logs, analytics); httpx.IsStatusError(err, http.StatusNotFound) != nil {
		// means we already uploaded the results.
		return discard(t.workload, t.metadata, idle(t.metadata))
	} else if err != nil {
		return failure(errorsx.Wrapf(err, "unable to upload completion: %s", t.workload.Id), newdelay(backoff.RandomFromRange(time.Second), t))
	}

	if t.cause != nil {
		return discard(t.workload, t.metadata, failure(errorsx.Wrap(t.cause, "work failed"), idle(t.metadata)))
	}

	return discard(t.workload, t.metadata, idle(t.metadata))
}

func discard(workload *Enqueued, md metadata, next state) statediscard {
	return statediscard{
		workload: workload,
		metadata: md,
		n:        next,
	}
}

type statediscard struct {
	metadata
	workload *Enqueued
	n        state
}

func (t statediscard) Update(ctx context.Context) state {
	defer func() {
		t.metadata.rm.Release(NewRuntimeResourcesFromDequeued(t.workload))
	}()

	if err := t.metadata.dirs.Completed(uuid.FromStringOrNil(t.workload.Id)); err != nil {
		return failure(errorsx.Wrap(err, "completion failed"), t.n)
	}

	// strictly probably not necessary, but ensure the workloads dont sync up by introducing a small randomized delay between workloads.
	return newdelay(backoff.DynamicHashDuration(100*time.Millisecond, strconv.FormatInt(int64(t.metadata.id), 36)), t.n)
}
