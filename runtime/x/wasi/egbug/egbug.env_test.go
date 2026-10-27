package egbug

import (
	"crypto/md5"
	"encoding/hex"
	"io/fs"
	"sort"
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultEnviron(t *testing.T) []string {
	raw, err := fs.ReadFile(embedded, "default.env")
	require.NoError(t, err)

	environ := []string{}
	for _, v := range strings.Split(string(raw), "\n") {
		if v != "" {
			environ = append(environ, v)
		}
	}

	return environ
}

func TestDefaultEnvMatchesDigest(t *testing.T) {
	environ := defaultEnviron(t)
	sort.Strings(environ)

	digest := md5.New()
	for _, v := range environ {
		digest.Write([]byte(v))
	}

	assert.Equal(t, defaultDigest, hex.EncodeToString(digest.Sum(nil)), "default.env and defaultDigest must be updated together")
}

// the environment of a baremetal run carries variables from the shell/sudo that launched it
// and baremetal specific variables. normalizing must reduce it to default.env.
func TestNormalizeEnvIgnoresLaunchVariance(t *testing.T) {
	expected := defaultEnviron(t)
	// values that differ per launch mode and replace the defaults.
	variance := []string{
		"EG_COMPUTE_GPU=true",
		"EG_COMPUTE_PROFILE_MODE=cpu",
		"EG_GIT_BASE_VCS=git@github.com:egdaemon/eg.git",
		"EG_COMPUTE_TTL=166666h40m0s",
		"EG_COMPUTE_WORKLOAD=main.wasm",
		"EG_COMPUTE_WORKSPACE_DIRECTORY=/workload/eg/.eg.workload/90cb0d5c/.eg.workspace",
		"EG_GIT_BASE_URI=https://github.com/egdaemon/eg.git",
		"EG_GIT_HEAD_URI=https://github.com/egdaemon/eg.git",
		"EG_GIT_HEAD_REF=main",
	}
	for _, v := range expected {
		k, val, _ := strings.Cut(v, "=")
		switch k {
		case _eg.EnvComputeWorkloadDirectory, _eg.EnvComputeWorkingDirectory, _eg.EnvComputeWorkspaceDirectory, _eg.EnvComputeCacheDirectory, _eg.EnvComputeRuntimeDirectory:
			t.Setenv(k, val)
		}
	}

	overridden := map[string]bool{}
	for _, v := range variance {
		k, _, _ := strings.Cut(v, "=")
		overridden[k] = true
	}

	launched := []string{}
	for _, v := range expected {
		if k, _, _ := strings.Cut(v, "="); !overridden[k] {
			launched = append(launched, v)
		}
	}

	environ := append(
		append(launched, variance...),
		"EG_COMPUTE_DEFAULT_GROUP=",
		"EG_COMPUTE_TLS_INSECURE=false",
		"EG_EXPERIMENTAL_BAREMETAL=true",
		"EG_GIT_AUTH_ACCESS_TOKEN=secret",
		"EG_GIT_AUTH_HTTP_USERNAME=user",
		"EG_GIT_AUTH_HTTP_PASSWORD=secret",
		"SSH_AUTH_SOCK=/eg.mnt/.eg.runtime/ssh.agent.socket",
		"LOGNAME=root",
		"SHELL=/bin/bash",
		"SHLVL=0",
		"SUDO_COMMAND=/usr/bin/bash -c eg compute baremetal tests/envvars",
		"SUDO_GID=0",
		"SUDO_HOME=/root",
		"SUDO_UID=0",
		"SUDO_USER=root",
		"USER=root",
		"_=/workload/.eg.workspace/eg",
	)

	actual, err := normalizeEnv(envx.Build().FromEnviron(environ...)).Environ()
	require.NoError(t, err)

	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}
