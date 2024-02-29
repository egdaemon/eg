package compile

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
)

func FromTranspiled(ctx context.Context, ws workspaces.Context, m ...transpile.Compiled) (modules []transpile.Compiled, err error) {
	modules = make([]transpile.Compiled, 0, len(m))

	for _, root := range m {
		var (
			path string
		)

		if path, err = filepath.Rel(ws.TransDir, root.Path); err != nil {
			return modules, err
		}

		path = workspaces.TrimRoot(path, filepath.Base(ws.GenModDir))
		path = workspaces.ReplaceExt(path, ".wasm")
		path = filepath.Join(ws.Root, ws.BuildDir, path)

		if !root.Generated {
			modules = append(modules, transpile.Compiled{Path: path, Generated: root.Generated})
		}

		if _, err = os.Stat(path); err == nil {
			// nothing to do.
			continue
		}

		if err = Run(ctx, ws.ModuleDir, root.Path, path); err != nil {
			return modules, err
		}
	}

	return modules, err
}

func Run(ctx context.Context, dir, module string, output string) (err error) {
	log.Println("compiling initiated", module, "->", output)
	defer log.Println("compiling completed", module, "->", output)

	if err = os.MkdirAll(filepath.Dir(output), 0750); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", output, strings.TrimPrefix(module, dir+"/"))
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	log.Println("compiling", cmd.String())

	return cmd.Run()
}
