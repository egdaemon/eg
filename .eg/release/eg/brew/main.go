package main

import (
	"context"
	egrel "eg/compute/release/eg"
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

// Baremetal command for darwin due to macosx nonsense for no cloud vms.
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	pattern := egrel.TarDarwin()
	runtime := shell.Runtime().
		EnvironFrom(eggolang.Env()...).
		Environ("CGO_ENABLED", "1").
		Environ("CGO_LDFLAGS", "-L/opt/homebrew/lib").
		Environ("GOBIN", filepath.Join(egtarball.Path(pattern), "bin"))

	log.Println("directory", egtarball.Path(pattern))
	log.Println("archive", egtarball.Archive(pattern))
	log.Println("release", eggithub.PatternVersion())

	err := eg.Perform(
		ctx,
		eg.Sequential(
			shell.Op(
				runtime.Newf("go install --tags duckdb_use_lib ./cmd/..."),
				runtime.Newf("tree -L 3 %s", egtarball.Path(pattern)),
			),
			egtarball.Pack(pattern),
			egtarball.SHA256Op(pattern),
			brew(pattern),
			eggithub.Draft(egtarball.Archive(pattern), formulapath()),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
