// Package main verifies a runner's environment has working gcp application default credentials
// issued through the egd-workloads identity (workload identity federation) with no user setup:
//
//	eg compute local tests/gcpaccess
//
// no token is printed, only whether one could be minted.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			shell.Op(
				// the runner environment must already point adc at the federated credential, no user setup.
				shell.New("test -n \"${GOOGLE_APPLICATION_CREDENTIALS}\""),
				shell.New("command -v gcloud || sudo sh /usr/share/eg/install/google-cloud.sh"),
				// exchanges the identity token via sts and impersonation, fails if the wif trust is not in place.
				shell.New("gcloud auth application-default print-access-token > /dev/null && echo 'gcp access ok'"),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
