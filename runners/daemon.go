package runners

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/interp/events"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewRunner(ctx context.Context, root, id string) (_ *Runner, err error) {
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

	if evtlog, err = events.NewLogEnsureDir(filepath.Join(workdir, "events")); err != nil {
		return nil, errors.WithStack(err)
	}

	r := &Runner{
		id:      id,
		workdir: workdir,
		control: control,
		evtlog:  evtlog,
		log:     log.New(logdst, id, log.Flags()),
	}
	go r.Background()

	return r, nil
}

type Runner struct {
	id      string
	workdir string
	control net.Listener
	log     *log.Logger
	evtlog  *events.Log
}

func (t Runner) Dial(ctx context.Context) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(t.workdir, "control.socket")), grpc.WithInsecure())
}

func (t Runner) Background() {
	log.Println("RUNNER INITIATED", t.id)
	defer log.Println("RUNNER COMPLETED", t.id)
	defer os.RemoveAll(t.workdir)

	srv := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()), // this is a local socket
	)

	events.NewServiceDispatch(t.evtlog).Bind(srv)

	if err := srv.Serve(t.control); err != nil {
		t.log.Println("runner shutdown", err)
		return
	}
}
