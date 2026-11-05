// guest used by the module set tests: runs one command from the runtime's module set through ffiwasi.Proxy and
// exits non-zero when its result is wrong.
package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/interp/execproxy"
	v2 "github.com/egdaemon/eg/interp/execproxy/v2"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiwasi"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WASI_MODE selects the check:
//
//	tee      - a command copies stdin to stdout and to a file relative to its directory.
//	env      - a command sees the environment of the request.
//	code     - a command's exit code is returned.
//	notfound - a command the set has no module for fails with codes.NotFound.
func main() {
	var (
		err    error
		code   int32
		b      []byte
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	run := func(stdin string, req *execproxy.ExecRequest) (int32, error) {
		listen := func(path string) (net.Listener, error) {
			return wasinet.Listen(ctx, "unix", path)
		}
		req.Dir = "."
		req.Environment = append(req.Environment, "PWD="+egenv.WorkingDirectory())
		return v2.Run(ctx, ffiwasi.Proxy{}, listen, egunsafe.RuntimeDirectory(), req, strings.NewReader(stdin), &stdout, &stderr)
	}

	switch mode := os.Getenv("WASI_MODE"); mode {
	case "tee":
		if code, err = run("hello world\n", &execproxy.ExecRequest{Cmd: "tee", Arguments: []string{"out"}}); err != nil || code != 0 {
			log.Fatalln("tee failed", code, err, stderr.String())
		}
		if b, err = os.ReadFile(egenv.WorkingDirectory("out")); err != nil || string(b) != "hello world\n" || stdout.String() != "hello world\n" {
			log.Fatalf("unexpected output file %q stdout %q %v\n", b, stdout.String(), err)
		}
	case "env":
		if code, err = run("", &execproxy.ExecRequest{Cmd: "env", Arguments: []string{"A"}, Environment: []string{"A=changed"}}); err != nil || code != 0 || stdout.String() != "changed\n" {
			log.Fatalln("unexpected result", code, err, stdout.String())
		}
	case "code":
		if code, err = run("", &execproxy.ExecRequest{Cmd: "code", Arguments: []string{"3"}}); err != nil || code != 3 {
			log.Fatalln("unexpected result", code, err)
		}
	case "notfound":
		if _, err = run("", &execproxy.ExecRequest{Cmd: "grep"}); status.Code(err) != codes.NotFound {
			log.Fatalln("unexpected result", err)
		}
	default:
		log.Fatalln("unknown mode", mode)
	}
}
