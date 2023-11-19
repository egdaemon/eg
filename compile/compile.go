package compile

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run(ctx context.Context, dir, module string, output string) (err error) {
	log.Println("compiling initiated", module, "->", output)
	defer log.Println("compiling completed", module, "->", output)

	if err = os.MkdirAll(filepath.Dir(output), 0750); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, strings.TrimPrefix(dir, module))
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	log.Println("compiling", cmd.String())

	return cmd.Run()
}
