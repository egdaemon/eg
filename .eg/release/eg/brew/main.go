package main

import (
	"context"
	egrel "eg/compute/release/eg"
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egarch"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

// Baremetal command for darwin due to macosx nonsense for no cloud vms.
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	// the archive and formula are arm64 only, an intel host would publish an x86_64 binary under an arm64 name.
	if arch := egarch.Host(); arch != "arm64" {
		log.Fatalln("release/eg/brew must run on an arm64 host, got", arch)
	}

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
			eg.When(egenv.Draft(), eggithub.Draft(egtarball.Archive(pattern), formulapath())),
			// the formula points at the release's download url, which only resolves once it is published.
			eg.When(egenv.Release(), tap(formulapath())),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
