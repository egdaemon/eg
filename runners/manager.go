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

func NewManager(ctx context.Context) *Manager {
	ctx, done := context.WithCancel(ctx)
	return &Manager{
		ctx:  ctx,
		done: done,
	}
}

type Manager struct {
	ctx  context.Context
	done context.CancelFunc
}

func (t Manager) NewRun(ctx context.Context, ws workspaces.Context, id string, options ...AgentOption) (*Agent, error) {
	return NewRunner(t.ctx, ws, id, options...), nil
}

func (t Manager) Dial(ctx context.Context, ws workspaces.Context) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(ws.Root, ws.RuntimeDir, "control.socket")), grpc.WithInsecure())
}
