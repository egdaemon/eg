package execx

import (
	"os"
	"os/exec"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
)

func MaybeRun(c *exec.Cmd) error {
	if c == nil {
		return nil
	}

	debugx.Println("---------------", errorsx.Must(os.Getwd()), "running", c.Dir, "->", c.String(), "---------------")
	return c.Run()
}
