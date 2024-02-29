package runners

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/egdaemon/eg/workspaces"
	"google.golang.org/grpc"
)

func DefaultManagerDirectory() string {
	return filepath.Join(workspaces.DefaultStateDirectory(), "daemons")
}

func DefaultRunnerDirectory(uid string) string {
	return filepath.Join(DefaultManagerDirectory(), uid)
}

func NewManager(ctx context.Context, dir string) *Manager {
	ctx, done := context.WithCancel(ctx)
	return &Manager{
		ctx:  ctx,
		done: done,
		dir:  dir,
	}
}

type Manager struct {
	dir  string
	ctx  context.Context
	done context.CancelFunc
}

func (t Manager) NewRun(ctx context.Context, ws workspaces.Context, id string, options ...AgentOption) (*Agent, error) {
	return NewRunner(t.ctx, t.dir, ws, id, options...)
}

func (t Manager) Dial(ctx context.Context, id string) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(t.dir, id, "control.socket")), grpc.WithInsecure())
}
