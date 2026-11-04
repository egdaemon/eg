package shell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/sh/v3/expand"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/egdaemon/sh/v3/syntax"
)

// invocation is a shell script the guest interprets in place of a bash or sh process.
type invocation struct {
	name   string // $0
	src    io.Reader
	params []string // set options, then "--" and the positional parameters
	stdin  io.Reader
}

// scripts interprets bash and sh invocations, and scripts whose shebang names either, in the guest with the
// command's directory, stdio and environment. the commands they run go back through scripts and then next.
// everything else, including scripts the guest cannot read, goes to next.
func scripts(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) (err error) {
		var (
			hc   = interp.HandlerCtx(ctx)
			inv  invocation
			ok   bool
			file *syntax.File
			r    *interp.Runner
		)

		if inv, ok = invoke(hc.Dir, hc.Stdin, args); !ok {
			return next(ctx, args)
		}

		if file, err = syntax.NewParser().Parse(inv.src, inv.name); err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.ExitStatus(2)
		}

		if r, err = interp.New(
			interp.Env(expand.ListEnviron(append(os.Environ(), changed(hc.Env)...)...)),
			interp.Dir(hc.Dir),
			interp.Params(inv.params...),
			interp.StdIO(inv.stdin, hc.Stdout, hc.Stderr),
			interp.OpenHandler(open),
			interp.ExecHandlers(scripts, func(interp.ExecHandlerFunc) interp.ExecHandlerFunc {
				return next
			}),
		); err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.ExitStatus(2)
		}

		return r.Run(ctx, file)
	}
}

// invoke reports the script args runs when args[0] is bash or sh, or a path to a script whose shebang names
// either. ok is false when the module has to run args.
func invoke(dir string, stdin io.Reader, args []string) (inv invocation, ok bool) {
	if !shell(args[0]) && strings.ContainsRune(args[0], '/') {
		if args, ok = shebang(resolve(dir, args[0]), args); !ok {
			return inv, false
		}
	}

	if !shell(args[0]) {
		return inv, false
	}

	return options(dir, stdin, args)
}

// options parses a bash or sh argv into the script it runs: the -c string, a script file, or stdin. ok is false
// for options the interpreter does not support, so bash in the module reports or handles them.
func options(dir string, stdin io.Reader, args []string) (inv invocation, ok bool) {
	var (
		command bool
		params  []string
		rest    []string
		b       []byte
		err     error
	)

	rest = args[1:]
flags:
	for len(rest) > 0 {
		switch arg := rest[0]; {
		case arg == "--":
			rest = rest[1:]
			break flags
		case arg == "-o" || arg == "+o":
			if len(rest) < 2 {
				return inv, false
			}
			params = append(params, arg, rest[1])
			rest = rest[2:]
		case len(arg) > 1 && (arg[0] == '-' || arg[0] == '+') && arg[1] != '-':
			for _, f := range arg[1:] {
				switch {
				case f == 'c' && arg[0] == '-':
					command = true
				case strings.ContainsRune("aefnux", f):
					params = append(params, string(arg[0])+string(f))
				default:
					return inv, false
				}
			}
			rest = rest[1:]
		default:
			break flags
		}
	}

	switch {
	case command:
		if len(rest) == 0 {
			return inv, false
		}
		inv.src, inv.name, inv.stdin = strings.NewReader(rest[0]), args[0], stdin
		if rest = rest[1:]; len(rest) > 0 {
			inv.name, rest = rest[0], rest[1:]
		}
	case len(rest) == 0:
		inv.src, inv.name = stdin, args[0]
		if stdin == nil {
			inv.src = strings.NewReader("")
		}
	default:
		if b, err = os.ReadFile(resolve(dir, rest[0])); err != nil {
			return inv, false
		}
		inv.src, inv.name, inv.stdin, rest = bytes.NewReader(b), rest[0], stdin, rest[1:]
	}

	inv.params = append(append(params, "--"), rest...)
	return inv, true
}

// shebang returns the argv that runs the script at path when its shebang names bash or sh, directly or through
// env. the path stays as args gave it, so the script sees the same $0.
func shebang(path string, args []string) (_ []string, ok bool) {
	var (
		f      *os.File
		rd     *bufio.Reader
		magic  []byte
		line   string
		fields []string
		err    error
	)

	if f, err = os.Open(path); err != nil {
		return nil, false
	}
	defer f.Close()

	rd = bufio.NewReader(f)
	if magic, err = rd.Peek(2); err != nil || string(magic) != "#!" {
		return nil, false
	}

	if line, err = rd.ReadString('\n'); err != nil && err != io.EOF {
		return nil, false
	}

	if fields = strings.Fields(line[2:]); len(fields) > 0 && filepath.Base(fields[0]) == "env" {
		fields = fields[1:]
	}

	if len(fields) == 0 || !shell(fields[0]) {
		return nil, false
	}

	return append(fields, args...), true
}

// shell reports whether name is bash or sh.
func shell(name string) bool {
	switch filepath.Base(name) {
	case "bash", "sh":
		return true
	default:
		return false
	}
}

// resolve returns path relative to dir unless it is absolute.
func resolve(dir string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(dir, path)
}
