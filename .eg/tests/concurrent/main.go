package main

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		eg.Parallel(
			eg.Module(
				ctx,
				eg.DefaultModule(),
				eg.Parallel(
					eg.Sequential(egbug.Log("pre1"), egbug.Sleep(1*time.Second), egbug.Log("post1")),
					eg.Sequential(egbug.Log("pre2"), egbug.Sleep(2*time.Second), egbug.Log("post2")),
					eg.Sequential(egbug.Log("pre3"), egbug.Sleep(1*time.Second), egbug.Log("post3")),
					eg.Sequential(egbug.Log("pre4"), egbug.Sleep(2*time.Second), egbug.Log("post4")),
				),
			),
			eg.Module(
				ctx,
				eg.DefaultModule(),
				eg.Parallel(
					eg.Sequential(egbug.Log("pre5"), egbug.Sleep(1*time.Second), egbug.Log("post5")),
					eg.Sequential(egbug.Log("pre6"), egbug.Sleep(2*time.Second), egbug.Log("post6")),
					eg.Sequential(egbug.Log("pre7"), egbug.Sleep(1*time.Second), egbug.Log("post7")),
					eg.Sequential(egbug.Log("pre8"), egbug.Sleep(2*time.Second), egbug.Log("post8")),
				),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
