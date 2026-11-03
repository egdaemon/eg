// guest used by the ffiexec tests: runs host commands through ffiexec.Stream from inside
// the wasi runtime and exits non-zero when their output or exit code is wrong.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiexec"
)

// STREAM_MODE selects the check:
//
//	output   - stdout and stderr arrive separately.
//	exit     - a non-zero exit code is returned, not an error.
//	stdin    - 4 MiB through cat round trips unchanged.
//	pipeline - one command's stdout feeds another's stdin through an io.Pipe in the guest.
func main() {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	run := func(stdin io.Reader, stdout io.Writer, cmd string, args ...string) int32 {
		code, err := ffiexec.Stream(ctx, ".", nil, cmd, args, stdin, stdout, &stderr)
		if err != nil {
			log.Fatalln("stream failed", cmd, err)
		}
		return code
	}

	switch mode := os.Getenv("STREAM_MODE"); mode {
	case "output":
		if code := run(nil, &stdout, "sh", "-c", "echo out; echo err >&2"); code != 0 {
			log.Fatalln("unexpected exit code", code)
		}
		if stdout.String() != "out\n" || stderr.String() != "err\n" {
			log.Fatalf("unexpected output %q %q\n", stdout.String(), stderr.String())
		}
	case "exit":
		if code := run(nil, &stdout, "sh", "-c", "exit 3"); code != 3 {
			log.Fatalln("unexpected exit code", code)
		}
	case "stdin":
		data := make([]byte, 4<<20)
		if _, err := rand.Read(data); err != nil {
			log.Fatalln(err)
		}
		if code := run(bytes.NewReader(data), &stdout, "cat"); code != 0 {
			log.Fatalln("unexpected exit code", code)
		}
		if !bytes.Equal(stdout.Bytes(), data) {
			log.Fatalf("round trip mismatch: sent %d bytes, received %d\n", len(data), stdout.Len())
		}
	case "pipeline":
		r, w := io.Pipe()
		produced := make(chan int32, 1)
		go func() {
			code := run(nil, w, "seq", "1", "200000")
			w.Close()
			produced <- code
		}()
		if code := run(r, &stdout, "wc", "-l"); code != 0 {
			log.Fatalln("unexpected consumer exit code", code)
		}
		if code := <-produced; code != 0 {
			log.Fatalln("unexpected producer exit code", code)
		}
		if strings.TrimSpace(stdout.String()) != "200000" {
			log.Fatalf("unexpected line count %q\n", stdout.String())
		}
	default:
		log.Fatalln("unknown mode", mode)
	}
}
