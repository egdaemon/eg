package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogentest"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

const (
	model = "unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF:Q4_K_M"

	style = `func TestContainerRunnerClone(t *testing.T) {
	t.Run("clone changes should not impact original", func(t *testing.T) {
		o := Container("derp")
		dup := o.Clone().OptionEnv("FOO", "BAR").Command("echo ${FOO}")
		require.Empty(t, o.options)
		require.Empty(t, o.cmd)
		require.Len(t, dup.options, 1)
		require.Equal(t, []string{"echo", "${FOO}"}, dup.cmd)
	})
}`
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		egllm.Prepare(egllm.Runner()),
		eg.Module(
			ctx,
			egllm.Runner(),
			llm,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}

func llm(ctx context.Context, o eg.Op) error {
	// stands in for egautogentest.Worst/Sample, which would otherwise
	// source this from recorded coverage data.
	seq := egautogentest.From(egautogentest.Fn{Path: egenv.WorkingDirectory("backoff", "backoff.go"), Name: "DynamicHashHour"})

	return egautogentest.Golang{Model: model, Style: style, Attempts: 3}.Generate(seq)(ctx, o)
}
