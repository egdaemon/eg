package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// must match the environment variable set by `eg compute cache exec`, a json encoded array of the command's arguments.
const envargs = "EG_CACHE_EXEC_ARGS"

// runs the command given by `eg compute cache exec` within the cache root.
// the arguments are quoted individually so they reach the command exactly as they were given.
func execute(ctx context.Context, op eg.Op) error {
	var args []string

	if err := json.Unmarshal([]byte(os.Getenv(envargs)), &args); err != nil {
		return fmt.Errorf("invalid %s: %w", envargs, err)
	}

	if len(args) == 0 {
		return errors.New("no command provided")
	}

	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", `'\''`)+"'")
	}

	return shell.Run(
		ctx,
		shell.Runtime().Directory(egenv.DirectoryCache()).Timeout(egenv.TTL()).New(strings.Join(quoted, " ")),
	)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		execute,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
