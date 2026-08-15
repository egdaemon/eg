package actl_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func TestBootstrapEnvDaemon(t *testing.T) {
	t.Run("account default resolves from vars_account_id at parse time", func(t *testing.T) {
		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvDaemon{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command"))
		require.Contains(t, out.String(), "EG_ACCOUNT=expected-account")
	})

	t.Run("secrets loaded ahead of parsing populate the account default, no subprocess involved", func(t *testing.T) {
		uri := "file://" + filepath.Join(t.TempDir(), "daemon.secret")

		var secretcontent bytes.Buffer
		require.NoError(t, envx.Build().Var("EG_ACCOUNT", "expected-account").CopyTo(&secretcontent))
		require.NoError(t, secrets.Update(t.Context(), uri, &secretcontent))

		// this is exactly what `cmdsecret.CmdEnv.Run()` does with the secret URIs before
		// exec'ing the wrapped command, and exactly what `eg secrets env -- eg actl
		// bootstrap-env daemon ...` relies on to populate EG_ACCOUNT in the child process's
		// environment before that child's main() builds kong.Vars.
		secretvars, err := envx.FromReader(secrets.NewReader(t.Context(), uri))
		require.NoError(t, err)
		require.Contains(t, secretvars, "EG_ACCOUNT=expected-account")

		var out bytes.Buffer
		genparser := cmdtestx.Genparser(actl.BootstrapEnvDaemon{},
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)),
		)

		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command"))
		require.Contains(t, out.String(), "EG_ACCOUNT=expected-account")
	})

	t.Run("no account available anywhere fails before Run ever executes", func(t *testing.T) {
		// deliberately omit vars_account_id: since AccountID's default is the template
		// ${vars_account_id}, kong must be able to resolve it when the parser is built, not
		// later inside Run() -- proving the value has to be in place ahead of flag
		// resolution, exactly what `eg secrets env -- eg actl bootstrap-env daemon ...`
		// exists to guarantee.
		genparser := cmdtestx.Genparser(actl.BootstrapEnvDaemon{}, cmdopts.RuntimeResources{}.KongVars())

		require.Panics(t, func() { genparser(t) })
	})
}
