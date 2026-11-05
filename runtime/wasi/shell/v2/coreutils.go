package shell

import (
	"context"
	"slices"

	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/sh/v3/interp"
)

// utilities are the commands coreutils.wasm runs: uutils 0.12.0's feat_wasm set, as coreutils --list prints it.
// nice and realpath are left out, since the module can neither start a process nor report its working directory.
var utilities = []string{
	"[", "arch", "b2sum", "base32", "base64", "basename", "basenc", "cat", "cksum", "comm", "cp", "csplit", "cut",
	"date", "dd", "dir", "dircolors", "dirname", "echo", "expand", "expr", "factor", "false", "fmt", "fold", "head",
	"hostid", "join", "link", "ln", "ls", "md5sum", "mkdir", "mktemp", "mv", "nl", "nproc", "numfmt", "od", "paste",
	"pathchk", "pr", "printenv", "printf", "ptx", "pwd", "readlink", "rm", "rmdir", "seq", "sha1sum", "sha224sum",
	"sha256sum", "sha384sum", "sha512sum", "shred", "shuf", "sleep", "sort", "split", "sum", "tail", "tee", "test",
	"touch", "tr", "true", "truncate", "tsort", "tty", "uname", "unexpand", "uniq", "unlink", "vdir", "wc", "yes",
}

// coreutils runs the utilities in the module's coreutils.wasm instead of as native binaries, and everything
// else goes to next. the module runs them itself, so they run as its user rather than the command's. a module
// without coreutils.wasm runs them natively.
func coreutils(c v1.ProxyClient) func(interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if !slices.Contains(utilities, args[0]) {
				return next(ctx, args)
			}

			return run(ctx, c, &v1.ExecRequest{Cmd: args[0], Arguments: args[1:], Wasm: "coreutils"})
		}
	}
}
