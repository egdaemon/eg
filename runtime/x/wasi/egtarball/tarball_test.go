package egtarball_test

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

func archive() string {
	return egtarball.GitPattern("example")
}

func Pack(ctx context.Context, op eg.Op) error {
	archive := egtarball.GitPattern("example")
	return eg.Perform(
		ctx,
		egtarball.Pack(archive),
		egtarball.SHA256Op(archive),
	)
}

func Populate(ctx context.Context, _ eg.Op) error {
	runtime := shell.Runtime().
		Environ("CONTENT", egenv.EphemeralDirectory("example.txt")).
		Environ("ARCHIVE", egtarball.Path(archive()))
	return shell.Run(
		ctx,
		runtime.Newf("printf \"hello world\n\" | tee ${CONTENT}"),
		runtime.Newf("cp ${CONTENT} ${ARCHIVE}"),
	)
}

func ExamplePack() {
	var (
		err error
	)

	ctx := context.Background()
	err = eg.Perform(
		ctx,
		Populate,
		Pack,
	)
	if err != nil {
		log.Fatalln(err)
	}
}
