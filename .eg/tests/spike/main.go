package main

import (
	"context"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	log.Println(egfs.Inspect(ctx, os.DirFS("/eg.mnt/.eg.runtime")))
	// log.Println(egfs.Inspect(ctx, os.DirFS("/")))
	log.Println("working", egenv.WorkingDirectory())
	log.Println("workload", egenv.WorkloadDirectory())

	log.Println(egfs.Inspect(ctx, os.DirFS(egenv.WorkingDirectory())))
	for _, v := range os.Environ() {
		log.Println(v)
	}

	runtime := shell.Runtime().As(egenv.User().Username)
	log.Println(egenv.User().Username)
	err := eg.Perform(
		ctx,
		shell.Op(
			runtime.Newf("ls -lha ."),
			runtime.Newf("echo %s", egenv.WorkingDirectory()),
			runtime.Newf("pwd"),
		),
		// eggit.AutoClone,
		// egbug.FileTree,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
