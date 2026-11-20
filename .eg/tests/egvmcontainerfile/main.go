// Package egvmcontainerfile provides tests for running modules in machines booted from a Containerfile.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egvm"
)

// Containerfile writes the default root container with a marker the machine's root must have.
func Containerfile(path string) eg.OpFn {
	return func(ctx context.Context, op eg.Op) (err error) {
		var (
			root []byte
		)

		eg.DefaultModule()
		if root, err = os.ReadFile(egunsafe.RuntimeDirectory("eg.default.module")); err != nil {
			return fmt.Errorf("unable to read the default root container: %w", err)
		}

		return os.WriteFile(path, append(root, []byte("\nRUN echo egvm > /etc/egvm.containerfile\n")...), 0600)
	}
}

// Marker checks the machine booted from the Containerfile's egiso, not the root's.
func Marker(ctx context.Context, op eg.Op) error {
	return shell.Run(ctx, shell.New("test -f /etc/egvm.containerfile"))
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	path := egunsafe.RuntimeDirectory("egvm.Containerfile")
	relative, err := filepath.Rel(egenv.DirectoryWorking(), path)
	if err != nil {
		log.Fatalln(err)
	}

	c := eg.Container("egvm.containerfile").BuildFromFile(path)

	if err = eg.Perform(
		ctx,
		Containerfile(path),
		eg.Module(ctx, egvm.New(egvm.OptionContainerfile(path)), Marker),
		eg.Module(ctx, egvm.New(egvm.OptionContainerfile(relative)), Marker),
		eg.Build(c),
		eg.Module(ctx, c, Marker),
	); err != nil {
		log.Fatalln(err)
	}
}
