// guest used by the cell tests: copies the image's /etc/hostname into the working directory, and with
// CELL_MACHINE runs uname -s in the machine, writing its output there too.
package main

import (
	"context"
	"log"
	"os"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegvm"
)

func main() {
	b, err := os.ReadFile("/etc/hostname")
	if err != nil {
		log.Fatalln(err)
	}

	if err = os.WriteFile(eg.DefaultWorkingDirectory("hostname"), b, 0600); err != nil {
		log.Fatalln(err)
	}

	if os.Getenv("CELL_MACHINE") == "" {
		return
	}

	cmd := fficoreutils.Reactor{Name: "the machine", Init: ffiegvm.Execveatinit, Wait: ffiegvm.Execveatwait}.Command(context.Background(), "sh", "-c", "uname -s > machine")
	cmd.Dir = eg.DefaultWorkingDirectory()
	cmd.Env = []string{"PATH=/bin"}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err = cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}
