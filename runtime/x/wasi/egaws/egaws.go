package egaws

import (
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// Debug returns a command that prints how the aws sdks and cli are configured for the user the command runs as: the user,
// home directory, the contents of ~/.aws, the aws environment (never the values of keys and tokens), and the identity the
// credentials resolve to. it never aborts the workload.
// c provides the environment and user the command runs with, e.g. shell.Run(ctx, egaws.Debug(console)).
func Debug(c shell.Command) shell.Command {
	return c.New(`id; echo HOME=${HOME}; tree -a -p -u -s ${HOME}/.aws; env | grep -E '^AWS_' | grep -v -E '^AWS_(SECRET_ACCESS_KEY|SESSION_TOKEN)='; aws sts get-caller-identity`).Debug().Lenient(true)
}
