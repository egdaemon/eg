package shell

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/james-lawrence/eg/runtime/wasi/env"
)

func Run(ctx context.Context, command string) (err error) {
	shell := env.String("bash", "EG_SHELL", "SHELL")
	path := env.String("/bin:/usr/bin", "PATH")
	log.Println("SHELL", shell)
	log.Println("PATH", path)
	cmd := exec.CommandContext(ctx, shell, "-c", command)
	cmd.Env = []string{
		fmt.Sprintf("SHELL=\"%s\"", shell),
		fmt.Sprintf("PATH=\"%s\"", path),
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if cmd.Dir, err = os.Getwd(); err != nil {
		return err
	}

	return cmd.Run()
}
