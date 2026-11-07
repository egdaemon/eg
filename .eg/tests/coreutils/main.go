// Package main exercises the egcoreutils reactor through fficoreutils, the reactor must be installed in the wasi
// module directory as egcoreutils.wasi (see cmd/eg/README.wasi.md).
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
	"google.golang.org/grpc/status"
)

func run(cmd *fficoreutils.Cmd) string {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return fmt.Sprintf("%q %q %s %v", stdout.String(), stderr.String(), status.Code(err), err)
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC | log.Ldate)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	log.Println("echo", run(fficoreutils.Command(ctx, "echo", "-n", "hello world")))

	tee := fficoreutils.Command(ctx, "tee", egenv.EphemeralDirectory("tee.out"))
	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte("hello "))
		pw.Write([]byte("world"))
		pw.Close()
	}()
	tee.Stdin = pr
	log.Println("tee", run(tee))
	b, err := os.ReadFile(egenv.EphemeralDirectory("tee.out"))
	log.Printf("tee file %q %v\n", b, err)

	var (
		wg      sync.WaitGroup
		results [2]string
	)
	posix := fficoreutils.Command(ctx, "echo", "-e", `a\tb`)
	posix.Env = []string{"POSIXLY_CORRECT=1"}
	wg.Go(func() { results[0] = run(posix) })
	wg.Go(func() { results[1] = run(fficoreutils.Command(ctx, "echo", "-e", `a\tb`)) })
	wg.Wait()
	log.Println("concurrent posix", results[0])
	log.Println("concurrent", results[1])

	log.Println("missing", run(fficoreutils.Command(ctx, "ls")))
}
