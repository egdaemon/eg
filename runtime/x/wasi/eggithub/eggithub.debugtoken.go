package eggithub

import (
	"context"
	"log"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// DebugToken logs what gh reports about the token the eggithub operations use: the account, where the token comes
// from, its kind and scopes. a debugging aid, it never fails, the token itself is not shown.
func DebugToken(ctx context.Context, o eg.Op) error {
	return debugtoken(shell.Runtime())(ctx, o)
}

func debugtoken(rt shell.Command) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		runtime := rt.Environ(_eg.EnvGithubToken, ffigit.Bearer())

		// gh exits non-zero when the token can't call /user, as installation tokens can't, and that message is
		// part of what is being looked for.
		if err := shell.Run(ctx, runtime.New("gh auth status").Lenient(true).Attempts(1)); err != nil {
			log.Println("gh auth status failed:", err)
		}

		return nil
	}
}
