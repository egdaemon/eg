package egbug

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Debug(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("env"),
		shell.New("tree -a -L 1 /opt"),
		shell.New("tree -a -L 2 /opt/egruntime"),
		shell.New("ls -lha /cache"),
		shell.New("ls -lha /root"),
	)
}
