package main

import (
	"context"
	"errors"
	"log"

	coreutils "github.com/egdaemon/eg/runtime/x/wasi/unsafecoreutils"
)

func main() {
	ctx := context.Background()
	if err := coreutils.Echo(ctx, "hello", "coreutils"); err != nil {
		log.Fatalln("echo", err)
	}
	if err := coreutils.Cat(ctx, "/work/a.txt"); err != nil {
		log.Fatalln("cat", err)
	}
	if err := coreutils.Ls(ctx, "/work"); err != nil {
		log.Fatalln("ls", err)
	}

	// reads this guest's stdin.
	if err := coreutils.Cat(ctx); err != nil {
		log.Fatalln("cat stdin", err)
	}

	var exit coreutils.ExitError
	if err := coreutils.Cat(ctx, "/work/missing"); !errors.As(err, &exit) || exit.Code == 0 {
		log.Fatalln("expected exit error", err)
	}
}
