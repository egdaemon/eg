package eggengolang

import (
	"context"
	"log"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egollama"
)

func ImproveTestCoverage(codeblock string, focus string, style string, usage string) eg.OpFn {
	const (
		model      = "qwen3-coder:30b"
		prompttmpl = `
		test the following codeblock using the given coding style and example usage blocks as guidance for how to structure the code.
rules:
- you must not include *any* comments
- you must not share variables between tests.
- you must not use or create mocking/stub or any kind of code that acts as a substitute that you do not find in samples.
- you must not do not write commented out code.
- you must not do not document the tests.
- you must omit the test and print all the code you skipped at the end if you can't write an effective test.
- you must ensure the test cases are comprehensive.

--------------------------------------------------- STYLE EXAMPLES ---------------------------------------------------
:sample:
--------------------------------------------------- USAGE EXAMPLES ---------------------------------------------------
:usage:
---------------------------------------------------   CODE BLOCK   ---------------------------------------------------
:codeblock:
---------------------------------------------------     FOCUS      ---------------------------------------------------
:focus:
`
	)

	prompt := strings.ReplaceAll(prompttmpl, ":sample:", style)
	prompt = strings.ReplaceAll(prompt, ":usage:", usage)
	prompt = strings.ReplaceAll(prompt, ":codeblock:", codeblock)
	prompt = strings.ReplaceAll(prompt, ":focus:", focus)

	return egollama.With(
		model,
		func(ctx context.Context, o eg.Op) error {
			result, err := egollama.Generate(ctx, egollama.New(), model, prompt)
			if err != nil {
				return err
			}

			log.Println("generated", result)
			return nil
		},
	)
}
