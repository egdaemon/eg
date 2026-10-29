package main

import (
	"context"
	"log"

	"eg/compute/daemon"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogentest"
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
			judge.With(daemon.Testgen(egautogentest.New().Judge(unmanaged).Limit(1).Attempts(16))),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}

// 01a188f7-50fb-7497-b1fa-2e90321f8429
// 01a188f9-6981-7511-949d-a270757e56d7
// 01a18905-7c0e-748d-9395-c7053efa45c1
