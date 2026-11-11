package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

// baremetal shell behavior only. runs without podman, no containers are available so only the host layout is verified.
// regression test for the container directory layout (/workload/...) leaking into the host,
// which surfaced as: chdir /workload/.eg.cache: no such file or directory
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		egbug.DebugFailure(
			eg.Sequential(
				egbug.EnsureEnv("2ff85b3300938e3efdb0567eb216de33", egbug.EgEnviron()...),
				shell.Op(
					// baremetal must not use the container directory layout.
					shell.Newf("test '%s' != '/workload/.eg.workspace'", egenv.DirectoryWorkload()),
					shell.Newf("test '%s' != '/workload/eg'", egenv.DirectoryWorking()),
					shell.Newf("test '%s' != '/workload/.eg.cache'", egenv.DirectoryCache()),
					shell.Newf("test '%s' != '/workload/.eg.runtime'", egenv.RuntimeDirectory()),
					// every directory must exist on the host.
					shell.Newf("test -d '%s'", egenv.DirectoryWorkload()),
					shell.Newf("test -d '%s'", egenv.DirectoryWorking()),
					shell.Newf("test -d '%s'", egenv.DirectoryCache()),
					shell.Newf("test -d '%s'", egenv.RuntimeDirectory()),
					// shell commands are executed on the host, they must be able to enter the directories.
					shell.Newf("test \"$(pwd -P)\" = \"$(realpath '%s')\"", egenv.DirectoryWorkload()).Directory(egenv.DirectoryWorkload()),
					shell.Newf("test \"$(pwd -P)\" = \"$(realpath '%s')\"", egenv.DirectoryWorking()).Directory(egenv.DirectoryWorking()),
					shell.Newf("test \"$(pwd -P)\" = \"$(realpath '%s')\"", egenv.DirectoryCache()).Directory(egenv.DirectoryCache()),
				),
			),
			egbug.Log("baremetal environment has drifted"),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
