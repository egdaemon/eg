package main

import (
	"context"
	"log"

	"eg/compute/daemon"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogen"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	judge := egllm.LLamaCPP("ggml-org/Qwen3.8-27B-GGUF:Q4_K_M").
		ReasoningBudget(4096).
		ReasoningEffort("medium")
	// the pool runs the judge once per function, so the functions use an unmanaged copy and the
	// server is started and stopped once around all of them.
	unmanaged := judge.Endpoint(egllm.Endpoint())
	c1 := eg.Container("eg.ubuntu")
	err := eg.Perform(
		ctx,
		eggithub.DebugToken,
		eggit.AutoClone,
		eg.Build(
			c1.BuildFromFile(".eg/Containerfile"),
		),
		eg.Module(
			ctx,
			c1,
			judge.With(daemon.Testgen(egautogen.New().Judge(unmanaged).Attempts(16))),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
