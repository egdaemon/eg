package main

import (
	"context"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogentest"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

const (
	model = "ggml-org/Qwen3.8-27B-GGUF:Q4_K_M"

	// number of the least covered functions to generate tests for.
	count = 5

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
		eggit.AutoClone,
		eg.Build(eg.DefaultModule()),
		egllm.Prepare(egllm.Runner()),
		eg.Module(
			ctx,
			egllm.Runner(),
			eg.Sequential(
				eggolang.AutoTest(
					eggolang.TestOption.Environ(os.Environ()...),
				),
				eggolang.RecordCoverage,
				egautogentest.New().Model(model).Attempts(3).Ensure(
					"no helper functions are added",
					"no global variables or setup code",
					"you must not change non-test code",
				).Run(egautogentest.Golang(egautogentest.Worst(count)).Style(style)),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
