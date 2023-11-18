package compile

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func Run(ctx context.Context, module string, output string) (err error) {
	log.Println("compiling initiated", module, "->", output)
	defer log.Println("compiling completed", module, "->", output)

	if err = os.MkdirAll(filepath.Dir(output), 0750); err != nil {
		return err
	}

	// cmd := exec.CommandContext(ctx, "tinygo", "build", "-tags", "wasm", "-target", "wasi", "-o", output, module)
	// cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, module)
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
