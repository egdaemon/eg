package faultharness

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egdaemon/eg/runtime/x/wasi/egautogentest"
	"github.com/egdaemon/eg/runtime/x/wasi/egpool"
)

// generator is the egautogentest.Generator for a single fault. it cannot reuse the golang generator because
// its rules and its verification expect a test that passes, where a reproduction is a test that fails
// with the reported error.
type generator struct {
	fault T
	// dir is the worktree the test is written to.
	dir string
}

// Seq is the function the fault was raised in.
func (t generator) Seq() egpool.Seq[egautogentest.Fn] {
	return egautogentest.From(egautogentest.Fn{Path: t.fault.Path, Name: t.fault.Func})
}

// Rules are the rules applied to the response, the ones given and what makes it a reproduction.
func (t generator) Rules(ensure ...string) []string {
	return append([]string{
		"respond with only a single complete go test file",
		"the test must fail with the reported error when run against the current code",
	}, ensure...)
}

// Prompt is the fault the model reproduces.
func (t generator) Prompt(c egautogentest.PromptContext) (string, error) {
	return fmt.Sprintf(
		"write a go test for %s in %s that reproduces the error with this stacktrace reported in production.\nline: %d\nstacktrace:\n%s\nrules:\n- %s",
		c.Fn.Name, c.Fn.Path, t.fault.Line, t.fault.Stacktrace,
		strings.Join(t.Rules(c.Ensure...), "\n- "),
	), nil
}

// Verify keeps the test only if it compiles and fails with the reported error.
//
// TODO: mock. overlay the artifact into t.dir, run the test for the package, and require a failure whose output
// matches the error at the top of the fault's stacktrace. nothing is left behind on error.
func (t generator) Verify(ctx context.Context, fn egautogentest.Fn, artifact string) error {
	return errors.New("faultharness.generator.Verify is not implemented")
}
