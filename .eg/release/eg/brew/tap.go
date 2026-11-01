package main

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

const (
	tapuri = "https://github.com/egdaemon/homebrew-tap-bin.git"
	// tapformula is where the formula lives within the tap.
	tapformula = "Formula/eg.rb"
)

// tap commits the rendered formula to the homebrew tap and pushes it. does nothing when the tap already has the formula.
// the token is the one used for the release, see ffigit.Bearer, it must be allowed to push to the tap.
// it is provided to git through the environment for the lifetime of the command only.
func tap(formula string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		runtime := shell.Runtime().
			Environ("TAP_URI", tapuri).
			Environ("TAP_DIR", egenv.DirectoryWorkload("tap")).
			Environ("TAP_FORMULA", formula).
			Environ("TAP_MESSAGE", fmt.Sprintf("eg %s", eggithub.PatternVersion())).
			Environ("GIT_AUTHOR_NAME", "egdaemon").
			Environ("GIT_AUTHOR_EMAIL", "noreply@egdaemon.com").
			Environ("GIT_COMMITTER_NAME", "egdaemon").
			Environ("GIT_COMMITTER_EMAIL", "noreply@egdaemon.com")

		if token := ffigit.Bearer(); token != "" {
			creds := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
			runtime = runtime.
				Environ("GIT_CONFIG_COUNT", 1).
				Environ("GIT_CONFIG_KEY_0", "http.extraheader").
				Environ("GIT_CONFIG_VALUE_0", "Authorization: Basic "+creds)
		}

		return shell.Run(ctx, runtime.New(`set -e
git clone --depth 1 "$TAP_URI" "$TAP_DIR"
cd "$TAP_DIR"
mkdir -p "$(dirname `+tapformula+`)"
cp "$TAP_FORMULA" `+tapformula+`
git add `+tapformula+`
if git diff --cached --quiet; then echo "tap is up to date"; exit 0; fi
git commit -m "$TAP_MESSAGE"
git push origin HEAD`))
	}
}
