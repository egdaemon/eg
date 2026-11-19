// Package egbash interprets shell commands inside the guest with egdaemon/sh, so expansion, redirects, pipes,
// builtins and the coreutils egdaemon/sh/x provides stay in the sandbox, and the other external commands run in
// the egcoreutils reactor. a workload opts in by importing egbash in place of eg/shell, whose API it implements.
package egbash

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
	"github.com/egdaemon/sh/v3/expand"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/egdaemon/sh/v3/syntax"
	"github.com/egdaemon/sh/x/coreutils"
)

// interpret interprets cmd in directory, relative to the working directory, with environ added to the guest's
// environment. external commands other than the coreutils go to do, with the exported variables that differ
// from the guest's environment.
func interpret(ctx context.Context, user string, group string, cmd string, directory string, environ []string, do Execer) (err error) {
	var (
		file *syntax.File
		r    *interp.Runner
	)

	if file, err = syntax.NewParser().Parse(strings.NewReader(cmd), ""); err != nil {
		return err
	}

	if !filepath.IsAbs(directory) {
		directory = egenv.WorkingDirectory(directory)
	}

	if r, err = interp.New(
		interp.Env(expand.ListEnviron(append(os.Environ(), environ...)...)),
		interp.Dir(directory),
		interp.StdIO(nil, os.Stdout, os.Stderr),
		interp.OpenHandler(open),
		interp.ExecHandlers(coreutils.ExecHandler, func(interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return func(ctx context.Context, args []string) error {
				hc := interp.HandlerCtx(ctx)
				return do(ctx, hc.Dir, changed(hc.Env), args[0], args[1:])
			}
		}),
	); err != nil {
		return err
	}

	return r.Run(ctx, file)
}

// reactor runs cmd in the egcoreutils reactor with the interpreter's stdio.
func reactor(ctx context.Context, dir string, environ []string, cmd string, args []string) error {
	hc := interp.HandlerCtx(ctx)
	c := fficoreutils.Command(ctx, cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), environ...)
	c.Stdin, c.Stdout, c.Stderr = hc.Stdin, hc.Stdout, hc.Stderr
	return exitstatus(c.Run())
}

// exitstatus reports a utility's non-zero exit as the interpreter's exit status, so || and $? see it. codes from
// ffierrors.ErrNotImplemented up are failures to run the utility, not exit codes, and are returned as they are.
func exitstatus(err error) error {
	if errno, ok := errors.AsType[syscall.Errno](err); ok && errno < ffierrors.ErrNotImplemented {
		return interp.ExitStatus(errno)
	}

	return err
}

// changed reports the exported variables whose value differs from the guest's own environment, the rest of an
// external command's environment is the guest's own. PWD and OLDPWD are left out, the command's directory
// sets them.
func changed(env expand.Environ) (environ []string) {
	env.Each(func(name string, vr expand.Variable) bool {
		switch name {
		case "PWD", "OLDPWD":
			return true
		}

		if !vr.Exported || vr.Kind != expand.String {
			return true
		}

		if v, ok := os.LookupEnv(name); ok && v == vr.Str {
			return true
		}

		environ = append(environ, name+"="+vr.Str)
		return true
	})

	return environ
}

type device struct {
	io.Reader
	io.WriteCloser
}

// open serves the /dev paths the guest has no mount for and opens everything else through wasi.
func open(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	hc := interp.HandlerCtx(ctx)
	switch path {
	case "/dev/null":
		return device{strings.NewReader(""), iox.WriteNopCloser(io.Discard)}, nil
	case "/dev/stdout":
		return device{strings.NewReader(""), iox.WriteNopCloser(hc.Stdout)}, nil
	case "/dev/stderr":
		return device{strings.NewReader(""), iox.WriteNopCloser(hc.Stderr)}, nil
	}

	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}
