package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	return shell.Run(
		ctx,
		shell.New("env"),
		shell.New("pwd"),
		shell.Newf("tree -L 2 %s", egenv.RuntimeDirectory()),
		shell.Newf("truncate --size 0 %s", egenv.RuntimeDirectory("environ.env")).Lenient(true),
		shell.Newf("tree -L 2 %s", egenv.RootDirectory()),
		shell.Newf("apt-get install stress"),
		shell.Newf("stress -t 5 -c %d", 24),
		shell.Newf("stress -t 5 -m %d", 24), // requires 6GB of ram
	)
}

func ChangeDetected(ctx context.Context, op eg.Op) error {
	log.Println("change detected")
	return nil
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	mods, err := eggit.NewModified(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	err = eg.Perform(
		ctx,
		Debug,
		eg.When(mods.Changed(egenv.RootDirectory()), ChangeDetected),
	)

	if err != nil {
		log.Fatalln(err)
	}

}
