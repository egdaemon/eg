// bootstrapped builds eg from the repository in the working directory, then runs `eg --hotswap` with the freshly
// built binary and `compute local` with the arguments of the module, EG_BOOTSTRAP_ARGS. it is what the repository's own
// workload does for its integration tests. use it to try a change to eg itself, the eg installed by the container's
// package doesn't have it. it runs in the container of the repository, see cmd/eg/compute/compute.builtin.bootstrap.go.
//
//	eg compute builtin bootstrap -vv tests/localref
package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

// path of the freshly installed eg binary within the workspace directory.
func sharedbin() string {
	return egenv.WorkspaceDirectory("eg")
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	var args []string
	if raw := egenv.Base64(nil, "EG_BOOTSTRAP_ARGS"); len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			log.Fatalln("unable to parse bootstrap arguments", err)
		}
	}

	log.Println("------------------------------- bootstrap args initiated -------------------------------")
	for _, v := range args {
		log.Println(v)
	}
	log.Println("------------------------------- bootstrap args completed -------------------------------")

	err := eg.Perform(
		ctx,
		eggolang.New().AutoInstall(),
		// copies the binary built by AutoInstall into the workspace directory so nested modules mount it.
		shell.Op(shell.Newf("install -m 0755 /home/egd/go/bin/eg %s", sharedbin())),
		egbug.Log("bootstrapped eg installed"),
		// privileged is required to access the podman socket, see the integration tests of the repository's workload.
		// the entrypoint is the shell's own, sudo as the user, without bash: each argument reaches eg as one argument.
		shell.Op(shell.Runtime().Privileged().Timeout(egenv.TTL()).New("eg --hotswap compute local").UnsafeEntrypoint(
			func(ctx context.Context, user, group, _ string, dir string, environ []string, do shell.Execer) error {
				argv := append([]string{"-E", "-H", "-u", user, "-g", group, sharedbin(), "--hotswap", "compute", "local"}, args...)
				return do(ctx, dir, environ, "sudo", argv)
			},
		)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
