package exec

import (
	context "context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

func NewExecProxy(root string) *ExecProxy {
	return &ExecProxy{
		dir: root,
	}
}

type ExecProxy struct {
	UnimplementedProxyServer
	dir string
}

func (t *ExecProxy) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

// Upload implements RunServer.
func (t *ExecProxy) Exec(ctx context.Context, req *ExecRequest) (resp *ExecResponse, err error) {
	var (
		cmd *exec.Cmd = exec.CommandContext(ctx, req.Cmd, req.Arguments...)
	)

	cmd.Dir = t.dir
	if !filepath.IsAbs(req.Dir) {
		cmd.Dir = filepath.Join(t.dir, req.Dir)
	}

	cmd.Env = append(req.Environment, cmd.Env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = execx.MaybeRun(cmd); err != nil {
		return nil, err
	}

	return &ExecResponse{}, nil
}
