package compile

import (
	"context"
	"log"
	"os"
	"os/exec"
)

func Run(ctx context.Context, module string, output string) error {
	log.Println("compiling initiated", module, "->", output)
	defer log.Println("compiling completed", module, "->", output)

	cmd := exec.CommandContext(ctx, "tinygo", "build", "-target", "wasi", "-wasm-abi", "generic", "-o", output, module)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
