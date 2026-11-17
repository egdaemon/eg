package execproxy

import (
	context "context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

// gracedelay is how long a cancelled command has to end before it is killed.
var gracedelay = 10 * time.Second

// ends makes the cancel of the command's context ask it to end, SIGTERM, instead of killing it, and kills it once it has had gracedelay.
// the command is told by SIGTERM because sudo, which the commands run under, does not relay a SIGINT sent from its own process group.
func ends(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = gracedelay
}

// NewExecProxy the commands it runs are ended, as ends describes, when ctx is done, however long their requests are.
func NewExecProxy(ctx context.Context, root string, environ []string) *ExecProxy {
	return &ExecProxy{
		ctx:     ctx,
		dir:     root,
		environ: environ,
	}
}

type ExecProxy struct {
	UnimplementedProxyServer
	ctx     context.Context
	dir     string
	environ []string
}

func (t *ExecProxy) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

// Upload implements RunServer.
func (t *ExecProxy) Exec(ctx context.Context, req *ExecRequest) (resp *ExecResponse, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(t.ctx, cancel)()

	var (
		cmd *exec.Cmd = exec.CommandContext(ctx, req.Cmd, req.Arguments...)
	)

	// cancelled, the command is asked to end and is killed when it has not within the delay, see ends.
	ends(cmd)

	cmd.Dir = req.Dir
	if !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(t.dir, cmd.Dir)
	}

	cmd.Env = append(t.environ, req.Environment...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = execx.MaybeRun(cmd); err != nil {
		return nil, err
	}

	return &ExecResponse{}, nil
}
