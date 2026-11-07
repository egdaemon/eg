// guest runs the utility named by its first argument through the egcoreutils reactor, printing what
// the utility wrote and how it exited.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func run(cmd *fficoreutils.Cmd) string {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if code := status.Code(err); code == codes.Unknown {
		return fmt.Sprintf("%q %q %s: %v", stdout.String(), stderr.String(), code, err)
	}
	return fmt.Sprintf("%q %q %s", stdout.String(), stderr.String(), status.Code(err))
}

func main() {
	ctx := context.Background()

	switch os.Args[1] {
	case "echo":
		fmt.Println(run(fficoreutils.Command(ctx, "echo", "-n", "hi")))
	case "tee":
		pr, pw := io.Pipe()
		go func() {
			pw.Write([]byte("ab"))
			pw.Write([]byte("c"))
			pw.Close()
		}()
		cmd := fficoreutils.Command(ctx, "tee", os.Args[2])
		cmd.Stdin = pr
		fmt.Println(run(cmd))
	case "concurrent":
		var (
			wg      sync.WaitGroup
			results [2]string
		)
		posix := fficoreutils.Command(ctx, "echo", "-e", `a\tb`)
		posix.Env = []string{"POSIXLY_CORRECT=1"}
		wg.Go(func() { results[0] = run(posix) })
		wg.Go(func() { results[1] = run(fficoreutils.Command(ctx, "echo", "-e", `a\tb`)) })
		wg.Wait()
		fmt.Println(results[0])
		fmt.Println(results[1])
	default:
		fmt.Println(run(fficoreutils.Command(ctx, os.Args[1])))
	}
}
