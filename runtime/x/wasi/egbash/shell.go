// Package egbash interprets shell commands inside the guest with egdaemon/sh, so expansion, redirects, pipes,
// builtins, the bash and sh scripts a command starts and the coreutils egdaemon/sh/x provides stay in the
// sandbox, and only the other external commands reach the module, through execproxy/v2. eg/shell runs through it
// with EG_EXPERIMENTAL_GUEST_SHELL, which requires EG_EXPERIMENTAL_PROC_SOCKETS for the module.
package egbash

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/iox"
	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/execproxy/v2"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/sh/v3/expand"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/egdaemon/sh/v3/syntax"
	"github.com/egdaemon/sh/x/coreutils"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/grpc"
)

// Run interprets cmd in directory, relative to the working directory, with environ added to the guest's
// environment. external commands other than the coreutils run in the module as user and group; when both are
// empty they run as the module's own user.
func Run(ctx context.Context, user string, group string, cmd string, directory string, environ []string) (err error) {
	var (
		file *syntax.File
		cc   *grpc.ClientConn
		r    *interp.Runner
	)

	if file, err = syntax.NewParser().Parse(strings.NewReader(cmd), ""); err != nil {
		return err
	}

	if !filepath.IsAbs(directory) {
		directory = egenv.WorkingDirectory(directory)
	}

	if cc, err = egunsafe.DialModuleControlSocket(ctx); err != nil {
		return err
	}
	defer cc.Close()

	if r, err = interp.New(
		interp.Env(expand.ListEnviron(append(os.Environ(), environ...)...)),
		interp.Dir(directory),
		interp.StdIO(nil, os.Stdout, os.Stderr),
		interp.OpenHandler(open),
		interp.ExecHandlers(scripts, coreutils.ExecHandler, func(interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return module(v1.NewProxyClient(cc), user, group)
		}),
	); err != nil {
		return err
	}

	return r.Run(ctx, file)
}

// module runs each external command in the module as user and group, in the interpreter's directory and with the
// environment it changed.
func module(c v1.ProxyClient, user string, group string) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) (err error) {
		var (
			hc = interp.HandlerCtx(ctx)
		)

		if user != "" || group != "" {
			args = append([]string{"sudo", "-E", "-H", "-u", user, "-g", group}, args...)
		}

		return run(ctx, c, &v1.ExecRequest{
			Dir:         egenv.PathRelativeWorking(hc.Dir),
			Cmd:         args[0],
			Arguments:   args[1:],
			Environment: changed(hc.Env),
		})
	}
}

// run executes req through c with the interpreter's stdio, and returns a non-zero exit as the interpreter's exit
// status.
func run(ctx context.Context, c v1.ProxyClient, req *v1.ExecRequest) (err error) {
	var (
		hc   = interp.HandlerCtx(ctx)
		code int32
	)

	listen := func(path string) (net.Listener, error) {
		return wasinet.Listen(ctx, "unix", path)
	}

	if code, err = execproxy.Run(ctx, c, listen, egunsafe.RuntimeDirectory(), req, hc.Stdin, hc.Stdout, hc.Stderr); err != nil {
		return err
	}

	if code != 0 {
		return interp.ExitStatus(code)
	}

	return nil
}

// changed reports the exported variables whose value differs from the guest's own environment. the module
// supplies the rest, so a variable the command never touched keeps the module's value. PWD and OLDPWD are guest
// paths, and the module derives them from the command's directory.
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
