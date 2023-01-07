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

	cmd := exec.CommandContext(ctx, "tinygo", "build", "-target", "wasi", "-wasm-abi", "generic", "-o", output, module)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
