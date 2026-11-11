package actl_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx/v2"
	"github.com/egdaemon/eg/cmd/eg/actl"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/secrets"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/require"
)

func TestBootstrapEnvDaemon(t *testing.T) {
	t.Run("account default resolves from vars_account_id at parse time", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

		require.NoError(t, harness.Run())
		require.Contains(t, out.String(), "EG_ACCOUNT=expected-account")
	})

	t.Run("secrets loaded ahead of parsing populate the account default, no subprocess involved", func(t *testing.T) {
		uri := "file://" + filepath.Join(t.TempDir(), "daemon.secret")

		var secretcontent bytes.Buffer
		require.NoError(t, envx.Build().Var(eg.EnvAccount, "expected-account").CopyTo(&secretcontent))
		require.NoError(t, secrets.Update(t.Context(), uri, &secretcontent))

		// this is exactly what `cmdsecret.CmdEnv.Run()` does with the secret URIs before
		// exec'ing the wrapped command, and exactly what `eg secrets env -- eg actl
		// bootstrap-env daemon ...` relies on to populate EG_ACCOUNT in the child process's
		// environment before that child's main() builds kong.Vars.
		secretvars, err := envx.FromReader(secrets.NewReader(t.Context(), uri))
		require.NoError(t, err)
		require.Contains(t, secretvars, "EG_ACCOUNT=expected-account")

		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

		require.NoError(t, harness.Run())
		require.Contains(t, out.String(), "EG_ACCOUNT=expected-account")
	})

	t.Run("no account available anywhere fails before Run ever executes", func(t *testing.T) {
		// deliberately omit vars_account_id: since AccountID's default is the template
		// ${vars_account_id}, kong must be able to resolve it when the parser is built, not
		// later inside Run() -- proving the value has to be in place ahead of flag
		// resolution, exactly what `eg secrets env -- eg actl bootstrap-env daemon ...`
		// exists to guarantee.
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(cmdopts.RuntimeResources{}.KongVars())

		_, err := harness.Parse()
		require.Error(t, err)
	})

	t.Run("lan settings", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			{name: "defaults", contains: []string{"EG_LAN_MDNS_ADVERTISE=false"}, absent: []string{"EG_LAN_PEER_TTL"}},
			{name: "enabled", args: []string{"--mdns-advertise", "--lan-peer-ttl", "48h"}, contains: []string{"EG_LAN_MDNS_ADVERTISE=true", "EG_LAN_PEER_TTL=48h0m0s"}},
			{name: "negated", args: []string{"--no-mdns-advertise"}, contains: []string{"EG_LAN_MDNS_ADVERTISE=false"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
					cmdopts.RuntimeResources{}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

				require.NoError(t, harness.Run(c.args...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})

	t.Run("vcs cache", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			{name: "defaults", contains: []string{"EG_COMPUTE_VCS_CACHE=false"}},
			{name: "enabled", args: []string{"--vcs-cache"}, contains: []string{"EG_COMPUTE_VCS_CACHE=true"}},
			{name: "negated", args: []string{"--no-vcs-cache"}, contains: []string{"EG_COMPUTE_VCS_CACHE=false"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
					cmdopts.RuntimeResources{}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

				require.NoError(t, harness.Run(c.args...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})

	t.Run("scheduling and api settings", func(t *testing.T) {
		// mirrors vars_cores_minimum_default from cmd/eg/main.go, varies by machine.
		cores := max(uint64(float64(runtime.NumCPU())*0.8), 1)

		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			// workers defaults to vars_cores_minimum_default, so it is always emitted.
			{name: "defaults", contains: []string{fmt.Sprintf("EG_COMPUTE_WORKLOAD_CAPACITY=%d", cores), "EG_COMPUTE_API_ENABLED=true"}, absent: []string{"EG_COMPUTE_COMPILE_CAPACITY", "EG_COMPUTE_WORKLOAD_TARGET_LOAD"}},
			{name: "workers", args: []string{"--workers", "2"}, contains: []string{"EG_COMPUTE_WORKLOAD_CAPACITY=2"}},
			{name: "enabled", args: []string{"--compilers", "3", "--target-load", "0.65"}, contains: []string{"EG_COMPUTE_COMPILE_CAPACITY=3", "EG_COMPUTE_WORKLOAD_TARGET_LOAD=0.65"}},
			{name: "api explicitly enabled", args: []string{"--api"}, contains: []string{"EG_COMPUTE_API_ENABLED=true"}},
			{name: "api disabled", args: []string{"--no-api"}, contains: []string{"EG_COMPUTE_API_ENABLED=false"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
					cmdopts.RuntimeResources{Cores: cores}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

				require.NoError(t, harness.Run(c.args...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})

	t.Run("labels are quoted", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
			cmdopts.RuntimeResources{Cores: uint64(runtime.NumCPU())}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

		require.NoError(t, harness.Run("--label", "fast disk", "--label", "prod"))
		require.Contains(t, out.String(), "EG_LABELS=\"fast disk,prod") // the detected gpu label may follow.
	})

	t.Run("cores and memory honor the requested values", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
			cmdopts.RuntimeResources{Cores: uint64(runtime.NumCPU())}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

		require.NoError(t, harness.Run("--cores", "1", "--memory", "4GiB"))
		require.Contains(t, out.String(), "EG_RESOURCES_CORES=1\n")
		require.Contains(t, out.String(), "EG_RESOURCES_MEMORY=\"4 GiB\"\n")
	})

	t.Run("physical resources cap the requested values", func(t *testing.T) {
		vm, err := mem.VirtualMemory()
		require.NoError(t, err)
		du, err := disk.Usage("/")
		require.NoError(t, err)

		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
			cmdopts.RuntimeResources{Cores: 1}.KongVars(),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

		require.NoError(t, harness.Run(
			"--cores", fmt.Sprint(runtime.NumCPU()+100),
			"--memory", "100PiB",
			"--disk", "100PiB",
		))
		require.Contains(t, out.String(), fmt.Sprintf("EG_RESOURCES_CORES=%d\n", runtime.NumCPU()))
		require.Contains(t, out.String(), fmt.Sprintf("EG_RESOURCES_MEMORY=\"%s\"\n", errorsx.Must(bytesx.Unit(vm.Total).MarshalText())))
		require.NotContains(t, out.String(), "EG_RESOURCES_DISK=\"100 PiB\"")
		require.Contains(t, out.String(), fmt.Sprintf("EG_RESOURCES_DISK=\"%s\"", errorsx.Must(bytesx.Unit(du.Total).MarshalText())))
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
				harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
					cmdopts.RuntimeResources{}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

				err := harness.Run(c.args...)
				if c.invalid {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Contains(t, out.String(), c.contains)
			})
		}
	})

	t.Run("custom environment variables are included", func(t *testing.T) {
		t.Setenv("BOOTSTRAP_TEST_BARE", "from-env")

		dir := t.TempDir()
		file := filepath.Join(dir, "extra.env")
		require.NoError(t, os.WriteFile(file, []byte("FROM_FILE=file\nOVERRIDDEN=file\n"), 0o600))

		for _, c := range []struct {
			name     string
			args     []string
			contains []string
			absent   []string
		}{
			{name: "none", absent: []string{"FOO=", "BOOTSTRAP_TEST_BARE", "FROM_FILE"}},
			{name: "explicit", args: []string{"-e", "FOO=bar"}, contains: []string{"FOO=bar\n"}},
			{name: "bare key is read from the process environment", args: []string{"--env", "BOOTSTRAP_TEST_BARE"}, contains: []string{"BOOTSTRAP_TEST_BARE=from-env"}},
			{name: "unset bare key is omitted", args: []string{"-e", "BOOTSTRAP_TEST_UNSET"}, absent: []string{"BOOTSTRAP_TEST_UNSET"}},
			{name: "repeated", args: []string{"-e", "A=1", "-e", "B=2"}, contains: []string{"A=1\n", "B=2\n"}},
			{name: "path", args: []string{"--envpath", file}, contains: []string{"FROM_FILE=file", "OVERRIDDEN=file"}},
			{name: "missing path is a noop", args: []string{"--envpath", filepath.Join(dir, "missing.env")}, absent: []string{"FROM_FILE"}},
			{name: "env wins over path", args: []string{"--envpath", file, "-e", "OVERRIDDEN=env"}, contains: []string{"OVERRIDDEN=file\nOVERRIDDEN=env\n"}},
		} {
			t.Run(c.name, func(t *testing.T) {
				var out bytes.Buffer
				harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
					cmdopts.RuntimeResources{}.KongVars(),
					kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
					kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
					kong.Writers(&out, nil),
					kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

				require.NoError(t, harness.Run(c.args...))
				for _, v := range c.contains {
					require.Contains(t, out.String(), v)
				}
				for _, v := range c.absent {
					require.NotContains(t, out.String(), v)
				}
			})
		}
	})

	t.Run("custom environment variables follow the generated ones", func(t *testing.T) {
		var out bytes.Buffer
		harness := cmdtestx.New(t, &actl.BootstrapEnvDaemon{}).Options(
			cmdopts.RuntimeResources{}.KongVars(),
			kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
			kong.Vars{"vars_account_id": "expected-account", "vars_entropy_seed": "expected-seed"},
			kong.Writers(&out, nil),
			kong.Bind(cmdopts.Entropy(cmdopts.GenerateEntropy)))

		require.NoError(t, harness.Run("-e", "EG_RESTART_ON_UPGRADE=no"))
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		require.Equal(t, "EG_RESTART_ON_UPGRADE=no", lines[len(lines)-1])
		require.Contains(t, lines, "EG_RESTART_ON_UPGRADE=auto")
	})
}
