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
	"github.com/james-lawrence/eg/interp/events"
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

func DefaultAgentClient(ctx context.Context) (cc *grpc.ClientConn, err error) {
	daemonpath := DefaultAgentSocketPath()
	if info, err := os.Stat(daemonpath); os.IsNotExist(err) {
		return nil, fmt.Errorf("agent not running at %s", info.Name())
	}

	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", daemonpath), grpc.WithInsecure(), grpc.WithBlock())
}

// local agent for managing jobs
func Agent(global *cmdopts.Global, grpcl net.Listener) (err error) {
	srv := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()), // this is a local socket
	)

	events.NewServiceAgent().Bind(srv)

	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()
		defer global.Shutdown()

		if err := srv.Serve(grpcl); err != nil {
			log.Println("grpc server failed", err)
		}
	}()

	return err
}
