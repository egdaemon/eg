package compute

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/jsonx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
)

// spooled runs a workload archive, as produced by `eg compute archive`, through the same
// spool and queue the daemon uses for workloads pushed by the upload client. its purpose is to
// exercise the runner path which `eg compute local` bypasses.
type spooled struct {
	cmdopts.RuntimeResources
	Archive string `arg:"" name:"archive" help:"workload archive to run, as produced by eg compute archive. '-' reads from stdin" default:"-"`
	Spool   string `name:"spool" help:"spool directory to use. defaults to a temporary directory removed on exit" default:""`
	Entry   string `name:"entry" help:"path of the compiled module within the archive, i.e. <module>/main.wasm. detected from the archive when blank" default:""`
}

// completion is the local equivalent of the control plane, it records the outcome of the workload
// and stops the queue.
type spooledcompletion struct {
	id     string
	cancel context.CancelFunc
	cause  error
	done   bool
}

func (t *spooledcompletion) Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader, analytics io.Reader) (err error) {
	if id != t.id {
		return nil
	}

	_, err = io.Copy(os.Stderr, logs)
	errorsx.Log(errorsx.Wrap(err, "unable to display logs"))

	t.cause, t.done = cause, true
	t.cancel()
	return nil
}

func (t spooled) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		uid      = uuid.Must(uuid.NewV7())
		archived *os.File
		agentopt = runners.AgentOptionNoop
	)

	tmpdir, err := os.MkdirTemp("", "eg.spooled.*")
	if err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() { errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory")) }()

	// isolate from the host's spool so we only ever run the workload from stdin.
	dirs := runners.NewSpoolDir(stringsx.FirstNonBlank(t.Spool, filepath.Join(tmpdir, "spool")))

	if archived, err = os.Create(filepath.Join(tmpdir, "archive.tar.gz")); err != nil {
		return errorsx.Wrap(err, "unable to create archive")
	}
	defer archived.Close()

	var src io.Reader = os.Stdin
	if t.Archive != "" && t.Archive != "-" {
		in, err := os.Open(t.Archive)
		if err != nil {
			return errorsx.Wrap(err, "unable to open archive")
		}
		defer in.Close()
		src = in
	}

	if _, err = io.Copy(archived, src); err != nil {
		return errorsx.Wrap(err, "unable to read archive")
	}

	if err = iox.Rewind(archived); err != nil {
		return errorsx.Wrap(err, "unable to rewind archive")
	}

	entry := t.Entry
	if entry == "" {
		if entry, err = spooledentry(archived); err != nil {
			return err
		}

		if err = iox.Rewind(archived); err != nil {
			return errorsx.Wrap(err, "unable to rewind archive")
		}
	}

	enq := &computeapi.Enqueued{
		Id:     uid.String(),
		Entry:  entry,
		Arch:   runtime.GOARCH,
		Cores:  t.RuntimeResources.Cores,
		Memory: uint64(t.RuntimeResources.Memory),
		Vram:   uint64(t.RuntimeResources.Vram),
	}

	// stage the workload identically to runners.DownloadClient.
	if err = dirs.Download(uid, "archive.tar.gz", archived); err != nil {
		return errorsx.Wrap(err, "unable to receive kernel archive")
	}

	encoded, err := jsonx.Marshal(&computeapi.EnqueuedDequeueResponse{Enqueued: enq})
	if err != nil {
		return errorsx.Wrap(err, "unable to encode metadata")
	}

	if err = dirs.Download(uid, "metadata.json", bytes.NewReader(encoded)); err != nil {
		return errorsx.Wrap(err, "unable to write metadata")
	}

	if err = dirs.Enqueue(uid); err != nil {
		return errorsx.Wrap(err, "unable to enqueue kernel archive")
	}

	// unlike the daemon, hotswapping mounts the binary running this command into the workload.
	if hotswapbin.String() != "" {
		agentopt = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	}

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	buildopts := []string{"--build-arg", envx.Format(eg.EnvBuildCommit, cmdopts.BuildCommit())}

	// nested containers cannot create the systemd scopes crun needs, build without a container runtime.
	if runners.Nested() {
		buildopts = append(buildopts, "--isolation", "chroot")
	}

	if err = runners.BuildRootContainer(ctx, buildopts...); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cc := &spooledcompletion{id: enq.Id, cancel: cancel}

	// a single worker, the queue's pool, capacity handling and recovery of interrupted workloads are not needed
	// to run the one workload staged above.
	err = runners.RunOne(
		ctx,
		0,
		0,
		httpx.NewFixedStatusClient(http.StatusNotImplemented),
		runners.NewResourceManager(runners.NewRuntimeResources()),
		&dirs,
		make(chan error, 1),
		runners.QueueOptionCompletion(cc),
		runners.QueueOptionAgentOptions(
			agentopt,
			// nested containers cannot create the systemd scopes crun needs, same as the runner for the containers it launches.
			envx.Toggle(runners.AgentOptionNoop, runners.AgentOptionCommandLine("--cgroups", "disabled"), runners.Nested()),
			// nested containers cannot write the sysctls netavark needs for a bridge network, share the outer network like compute local.
			envx.Toggle(runners.AgentOptionNoop, runners.AgentOptionCommandLine("--network", "host"), runners.Nested() && !envx.Boolean(false, eg.EnvExperimentalDisableHostNetwork)),
		),
		runners.QueueOptionLogVerbosity(gctx.Verbosity),
	)

	if cc.done {
		return errorsx.Wrap(cc.cause, "workload failed")
	}

	return errorsx.Compact(err, errorsx.Errorf("queue stopped before the workload completed"))
}

// spooledentry locates the compiled module within the archive.
func spooledentry(r io.Reader) (_ string, err error) {
	var (
		found []string
		gzr   *gzip.Reader
	)

	if gzr, err = gzip.NewReader(r); err != nil {
		return "", errorsx.Wrap(err, "unable to read archive")
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", errorsx.Wrap(err, "unable to read archive")
		}

		if header.Typeflag == tar.TypeReg && strings.HasSuffix(header.Name, ".wasm") {
			found = append(found, filepath.Clean(header.Name))
		}
	}

	if len(found) != 1 {
		return "", errorsx.Errorf("unable to detect entry point, found %v, use --entry", found)
	}

	return found[0], nil
}
