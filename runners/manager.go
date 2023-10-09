package runners

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/envx"
	"google.golang.org/grpc"
)

func DefaultManagerDirectory() string {
	return filepath.Join(envx.String(os.TempDir(), "STATE_DIRECTORY", "XDG_STATE_HOME"), "daemons")
}

func NewManager(ctx context.Context, dir string) *Manager {
	ctx, done := context.WithCancel(ctx)
	log.Println("DERP", dir)
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

func (t Manager) NewRun(ctx context.Context, id string) (*Agent, error) {
	return NewRunner(t.ctx, t.dir, id)
}

func (t Manager) Dial(ctx context.Context, id string) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(ctx, fmt.Sprintf("unix://%s", filepath.Join(t.dir, id, "control.socket")), grpc.WithInsecure())
}
