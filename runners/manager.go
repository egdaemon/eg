package runners

import (
	"context"
	"path/filepath"

	"github.com/egdaemon/eg/workspaces"
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
