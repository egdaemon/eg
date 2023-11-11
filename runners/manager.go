package runners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/workspaces"
	"google.golang.org/grpc"
)

func DefaultManagerDirectory() string {
	return filepath.Join(envx.String(os.TempDir(), "STATE_DIRECTORY", "XDG_STATE_HOME"), "daemons")
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

func (t Manager) NewRun(ctx context.Context, ws workspaces.Context, id string) (*Agent, error) {
	return NewRunner(t.ctx, t.dir, ws, id)
}

func (t Manager) Dial(ctx context.Context, id string) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(t.dir, id, "control.socket")), grpc.WithInsecure())
}
