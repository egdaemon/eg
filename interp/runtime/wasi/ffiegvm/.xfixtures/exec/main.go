// guest used by the ffiegvm tests: runs tool a b in the machine with its own stdio, printing why it could not.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegvm"
)

func main() {
	cmd := fficoreutils.Reactor{Name: "the machine", Init: ffiegvm.Execveatinit, Wait: ffiegvm.Execveatwait}.Command(context.Background(), "tool", "a", "b")
	cmd.Dir = "/srv"
	cmd.Env = []string{"A=1"}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
