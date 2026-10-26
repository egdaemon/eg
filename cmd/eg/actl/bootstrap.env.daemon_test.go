package actl_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
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

	t.Run("lan settings are only emitted when requested", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			{name: "defaults", absent: []string{"EG_MDNS_ADVERTISE", "EG_LAN_PEER_TTL"}},
			{name: "enabled", args: []string{"--mdns-advertise", "--lan-peer-ttl", "48h"}, contains: []string{"EG_MDNS_ADVERTISE=true", `EG_LAN_PEER_TTL="48h0m0s"`}},
			{name: "negated", args: []string{"--no-mdns-advertise"}, absent: []string{"EG_MDNS_ADVERTISE"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				genparser := cmdtestx.Genparser(actl.BootstrapEnvDaemon{},
					cmdopts.RuntimeResources{}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)),
				)

				require.NoError(t, cmdtestx.Execute(t, genparser(t), append([]string{"command"}, c.args...)...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})

	t.Run("scheduling and api settings are only emitted when requested", func(t *testing.T) {
		// mirrors vars_cores_minimum_default from cmd/eg/main.go, varies by machine.
		cores := max(uint64(float64(runtime.NumCPU())*0.8), 1)

		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			// workers defaults to vars_cores_minimum_default, so it is always emitted.
			{name: "defaults", contains: []string{fmt.Sprintf("EG_COMPUTE_WORKLOAD_CAPACITY=%d", cores)}, absent: []string{"EG_COMPUTE_COMPILE_CAPACITY", "EG_COMPUTE_WORKLOAD_TARGET_LOAD", "EG_COMPUTE_API_ENABLED"}},
			{name: "workers", args: []string{"--workers", "2"}, contains: []string{"EG_COMPUTE_WORKLOAD_CAPACITY=2"}},
			{name: "enabled", args: []string{"--compilers", "3", "--target-load", "0.65"}, contains: []string{"EG_COMPUTE_COMPILE_CAPACITY=3", "EG_COMPUTE_WORKLOAD_TARGET_LOAD=0.65"}, absent: []string{"EG_COMPUTE_API_ENABLED"}},
			{name: "api explicitly enabled", args: []string{"--api"}, absent: []string{"EG_COMPUTE_API_ENABLED"}},
			{name: "api disabled", args: []string{"--no-api"}, contains: []string{"EG_COMPUTE_API_ENABLED=false"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				genparser := cmdtestx.Genparser(actl.BootstrapEnvDaemon{},
					cmdopts.RuntimeResources{Cores: cores}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)),
				)

				require.NoError(t, cmdtestx.Execute(t, genparser(t), append([]string{"command"}, c.args...)...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})

	t.Run("restart on upgrade", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			args     []string
			contains string
			invalid  bool
		}{
			{name: "default", contains: "EG_RESTART_ON_UPGRADE=auto"},
			{name: "yes", args: []string{"--restart-on-upgrade", "yes"}, contains: "EG_RESTART_ON_UPGRADE=yes"},
			{name: "no", args: []string{"--restart-on-upgrade", "no"}, contains: "EG_RESTART_ON_UPGRADE=no"},
			{name: "invalid", args: []string{"--restart-on-upgrade", "sometimes"}, invalid: true},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				genparser := cmdtestx.Genparser(actl.BootstrapEnvDaemon{},
					cmdopts.RuntimeResources{}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)),
				)

				err := cmdtestx.Execute(t, genparser(t), append([]string{"command"}, c.args...)...)
				if c.invalid {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Contains(t, out.String(), c.contains)
			})
		}
	})
}
