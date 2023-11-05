package runners

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/interp/c8s"
	"github.com/james-lawrence/eg/interp/events"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func DefaultRunnerRuntimeDir() string {
	return filepath.Join("/", "opt", "egruntime")
}

func DefaultRunnerSocketPath() string {
	return filepath.Join(DefaultRunnerRuntimeDir(), "control.socket")
}

func NewRunner(ctx context.Context, root, id string) (_ *Agent, err error) {
	var (
		control net.Listener
		workdir = filepath.Join(root, id)
		logdst  *os.File
		evtlog  *events.Log
	)

	if err = os.MkdirAll(workdir, 0700); err != nil {
		return nil, errors.WithStack(err)
	}

	if logdst, err = os.Create(filepath.Join(workdir, "daemon.log")); err != nil {
		return nil, errors.WithStack(err)
	}

	if control, err = net.Listen("unix", filepath.Join(workdir, "control.socket")); err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		control.Close()
	}()

	if evtlog, err = events.NewLogEnsureDir(events.NewLogDirFromRunID(root, id)); err != nil {
		return nil, errors.WithStack(err)
	}

	r := &Agent{
		id:      id,
		workdir: workdir,
		control: control,
		evtlog:  evtlog,
		srv: grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
		),
		blocked: make(chan struct{}),
		log:     log.New(logdst, id, log.Flags()),
	}
	go r.background()

	return r, nil
}

type Agent struct {
	id      string
	workdir string
	control net.Listener
	srv     *grpc.Server
	log     *log.Logger
	blocked chan struct{}
	evtlog  *events.Log
}

func (t Agent) Dial(ctx context.Context) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(t.workdir, "control.socket")), grpc.WithInsecure())
}

func (t Agent) Close() error {
	log.Println("graceful shutdown initiated")
	t.srv.GracefulStop()
	<-t.blocked
	return nil
}

func (t Agent) background() {
	log.Println("RUNNER INITIATED", t.id)
	log.Println("working directory", t.workdir)
	log.Println("control socket", t.control.Addr().String())
	defer close(t.blocked)
	defer log.Println("RUNNER COMPLETED", t.id)
	defer os.RemoveAll(t.workdir)

	events.NewServiceDispatch(t.evtlog).Bind(t.srv)
	c8s.NewServiceProxy(
		t.workdir,
		c8s.ServiceProxyOptionEnviron(
			append(
				os.Environ(),
				fmt.Sprintf("CI=%s", envx.String("", "EG_CI", "CI")),
				fmt.Sprintf("EG_CI=%s", envx.String("", "EG_CI", "CI")),
				fmt.Sprintf("EG_RUN_ID=%s", t.id),
				fmt.Sprintf("EG_ROOT_DIRECTORY=%s", t.workdir),
				fmt.Sprintf("EG_CACHE_DIRECTORY=%s", envx.String("derp0", "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY")),
				fmt.Sprintf("EG_RUNTIME_DIRECTORY=%s", "derp1"),
				fmt.Sprintf("RUNTIME_DIRECTORY=%s", "derp2"),

				// fmt.Sprintf("EG_CACHE_DIRECTORY=%s", envx.String(guestcachedir, "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY")),
				// fmt.Sprintf("EG_RUNTIME_DIRECTORY=%s", guestruntimedir),
				// fmt.Sprintf("RUNTIME_DIRECTORY=%s", guestruntimedir),
			)...,
		),
	).Bind(t.srv)
	// TODO: container endpoint.
	// enable event logging.
	// events.NewServiceAgent(
	// 	langx.Must(filepath.Abs(DefaultManagerDirectory())),
	// ).Bind(srv)

	if err := t.srv.Serve(t.control); err != nil {
		t.log.Println("runner shutdown", err)
		return
	}
}
