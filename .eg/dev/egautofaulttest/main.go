package main

import (
	"context"
	"log"

	"eg/compute/daemon"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogentest"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	judge := egllm.LLamaCPP("ggml-org/Qwen3.8-27B-GGUF:Q4_K_M").
		ReasoningBudget(4096).
		ReasoningEffort("medium")
	c1 := eg.Container("eg.ubuntu")
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(
			c1.BuildFromFile(".eg/Containerfile"),
		),
		eg.Module(
			ctx,
			c1,
			daemon.Faulttest(egautogentest.New().Judge(judge).Attempts(16)),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
