// Package gitclone provides tests for eggit.AutoClone and eggit.Clone with a destination, on the host and within a container.
// when cloning is disabled (eg compute local) there is nothing to verify beyond the clone being skipped.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

// verify the working directory is a repository at the commit of the environment, this is the result of AutoClone.
func verifyautoclone(ctx context.Context, _ eg.Op) error {
	if !egenv.CloneEnabled() {
		log.Println("cloning disabled, skipping verification")
		return nil
	}

	return shell.Run(
		ctx,
		shell.Runtime().Directory(egenv.DirectoryWorking()).Newf(
			"test \"$(git rev-parse HEAD)\" = %s", eggit.EnvCommit().Hash,
		),
	)
}

// verify the clone of the repository of the environment into a directory of the cache is at the commit.
func verifycloneto(ctx context.Context, _ eg.Op) error {
	if !egenv.CloneEnabled() {
		log.Println("cloning disabled, skipping verification")
		return nil
	}

	return shell.Run(
		ctx,
		shell.Runtime().Directory(clonedir()).Newf(
			"test \"$(git rev-parse HEAD)\" = %s", eggit.EnvCommit().Hash,
		),
	)
}

func clonedir() string {
	return egenv.DirectoryCache("gitclone", eggit.EnvCommit().Hash.String())
}

// clone the repository of the environment into a directory within the cache, skipped when cloning is disabled.
func cloneop(ctx context.Context, op eg.Op) error {
	if !egenv.CloneEnabled() {
		log.Println("cloning disabled, skipping clone")
		return nil
	}

	return eggit.CloneOp(clonedir(), eggit.EnvCanonicalURI(), "origin", eggit.EnvCommit().Hash.String())(ctx, op)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		egbug.Log("gitclone initiated"),
		verifyautoclone,
		cloneop,
		verifycloneto,
		egbug.Log("gitclone successful"),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
