package daemons

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/interp/events"
	"github.com/james-lawrence/eg/runners"
	"github.com/james-lawrence/eg/runtime/wasi/langx"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func DefaultAgentSocketPath() string {
	return filepath.Join(envx.String(os.TempDir(), "STATE_DIRECTORY"), "agent.socket")
}

func DefaultAgentListener() (n net.Listener, err error) {
	daemonpath := DefaultAgentSocketPath()
	if info, err := os.Stat(daemonpath); !os.IsNotExist(err) {
		return nil, fmt.Errorf("agent already running at %s", info.Name())
	}

	log.Println("spawning host agent", daemonpath)
	if n, err = net.Listen("unix", daemonpath); err != nil {
		return nil, errors.WithStack(err)
	}

	return n, err
}

func MaybeAgentListener() (n net.Listener, err error) {
	daemonpath := DefaultAgentSocketPath()
	if _, err := os.Stat(daemonpath); !os.IsNotExist(err) {
		log.Println("agent already running at", daemonpath)
		return nil, nil
	}

	log.Println("spawning host agent", daemonpath)
	if n, err = net.Listen("unix", daemonpath); err != nil {
		return nil, errors.WithStack(err)
	}

	return n, err
}

func DefaultRunnerClient(ctx context.Context) (cc *grpc.ClientConn, err error) {
	daemonpath := runners.DefaultRunnerSocketPath()
	log.Println("connecting", daemonpath)
	if _, err := os.Stat(daemonpath); os.IsNotExist(err) {
		return nil, fmt.Errorf("agent not running at %s", daemonpath)
	}
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", daemonpath), grpc.WithInsecure(), grpc.WithBlock())
}

func AutoRunnerClient(global *cmdopts.Global, uid string) (cc *grpc.ClientConn, err error) {
	var (
		ragent *runners.Agent
	)

	if cc, err = DefaultRunnerClient(global.Context); err == nil {
		return cc, nil
	}

	log.Println("initializing runner", uid)
	m := runners.NewManager(
		global.Context,
		langx.Must(filepath.Abs(runners.DefaultManagerDirectory())),
	)

	if ragent, err = m.NewRun(global.Context, uid); err != nil {
		return nil, err
	}

	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()
		<-global.Context.Done()
		errorsx.MaybeLog(errorsx.Wrap(ragent.Close(), "unable to cleanly stop runner"))
	}()

	return ragent.Dial(global.Context)
}

// local agent for managing jobs
func Agent(global *cmdopts.Global, grpcl net.Listener) (err error) {
	srv := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()), // this is a local socket
	)

	events.NewServiceAgent(
		langx.Must(filepath.Abs(runners.DefaultManagerDirectory())),
	).Bind(srv)

	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()

		if err := srv.Serve(grpcl); err != nil {
			log.Println("grpc server failed", err)
		}
	}()

	go func() {
		<-global.Context.Done()
		log.Println("shutting down agent service")
		srv.GracefulStop()
		grpcl.Close()
	}()

	return err
}
