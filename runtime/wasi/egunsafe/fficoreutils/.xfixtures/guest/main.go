// guest runs the utility named by its first argument through the egcoreutils reactor, printing what
// the utility wrote and how it exited.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

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
	case "stream":
		// tee streams when its output for the first chunk arrives before the rest of stdin is written.
		pr, pw := io.Pipe()
		out := &signal{seen: make(chan struct{})}
		go func() {
			pw.Write([]byte("ab"))
			select {
			case <-out.seen:
				pw.Write([]byte("c"))
			case <-time.After(5 * time.Second):
				pw.Write([]byte("-timeout"))
			}
			pw.Close()
		}()
		cmd := fficoreutils.Command(ctx, "tee")
		cmd.Stdin, cmd.Stdout = pr, out
		err := cmd.Run()
		fmt.Printf("%q %s\n", out.String(), status.Code(err))
	case "tr":
		cmd := fficoreutils.Command(ctx, "tr", "a-z", "A-Z")
		cmd.Stdin = strings.NewReader("hello\n")
		fmt.Println(run(cmd))
	case "pwd":
		cmd := fficoreutils.Command(ctx, "pwd")
		cmd.Dir = os.Args[2]
		fmt.Println(run(cmd))
	default:
		fmt.Println(run(fficoreutils.Command(ctx, os.Args[1])))
	}
}

// signal is a buffer that closes seen on its first write. the buffer is not embedded: its ReadFrom
// would let io.Copy bypass Write.
type signal struct {
	buf  bytes.Buffer
	seen chan struct{}
	once sync.Once
}

func (t *signal) Write(p []byte) (int, error) {
	defer t.once.Do(func() { close(t.seen) })
	return t.buf.Write(p)
}

func (t *signal) String() string {
	return t.buf.String()
}
