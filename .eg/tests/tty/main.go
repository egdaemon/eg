package main

import (
	"context"
	"crypto/md5"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

// Ensure that the environment is as expected.
func EnvTest(ctx context.Context, _ eg.OpFn) error {
	const expected = ""
	environ := os.Environ()
	sort.Stable(sort.StringSlice(environ))
	digest := md5.Sum([]byte(strings.Join(environ, "")))
	return shell.Run(
		ctx,
		shell.New("env | sort"),
		shell.Newf("test %s -eq %s", digest, expected),
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		egbug.SystemInit,
		egbug.Env,
		egbug.Users,
		egbug.FileTree,
		eg.Build(eg.DefaultModule()),
		eg.Module(ctx, eg.DefaultModule(), egbug.SystemInit),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
