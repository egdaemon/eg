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

	// stands in for egautogentest.Worst/Sample, which would otherwise
	// source this from recorded coverage data.
	var seq = egautogentest.FromSpecified(egautogentest.Fn{Path: egenv.WorkingDirectory("backoff", "backoff.go"), Name: "DynamicHashHour"})
	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		egllm.Prepare(egllm.Runner()),
		eg.Module(
			ctx,
			egllm.Runner(),
			egautogentest.New().Judge(egllm.LLamaCPP(model)).Attempts(3).Ensure(
				"no helper functions are added",
				"no global variables or setup code",
				"you must not change non-test code",
			).Run(egautogentest.Golang(seq).Templates(style)),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
