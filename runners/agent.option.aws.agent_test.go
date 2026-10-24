package runners

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/awsx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestAgentOptionAWSCredentials(t *testing.T) {
	// an environment whose credentials come from the variables only.
	isolate := func(t *testing.T) {
		t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing"))
		t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing"))
		t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
		for _, k := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE", "AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE"} {
			t.Setenv(k, "")
			require.NoError(t, os.Unsetenv(k))
		}
	}

	t.Run("should provide the resolved credentials to the workload", func(t *testing.T) {
		isolate(t)
		t.Setenv("AWS_ACCESS_KEY_ID", "akid")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
		t.Setenv("AWS_SESSION_TOKEN", "token")

		envb := envx.Build()
		opt, err := AgentOptionAWSCredentials(t.Context(), envb)
		require.NoError(t, err)

		environ, err := envb.Environ()
		require.NoError(t, err)

		var found string
		for _, kv := range environ {
			if len(kv) > len(eg.EnvUnsafeAWSCredentialsB64) && kv[:len(eg.EnvUnsafeAWSCredentialsB64)+1] == eg.EnvUnsafeAWSCredentialsB64+"=" {
				found = kv[len(eg.EnvUnsafeAWSCredentialsB64)+1:]
			}
		}
		require.NotEmpty(t, found)

		raw, err := base64.URLEncoding.DecodeString(found)
		require.NoError(t, err)
		var creds awsx.StaticCredentials
		require.NoError(t, json.Unmarshal(raw, &creds))
		require.Equal(t, awsx.StaticCredentials{AccessKeyID: "akid", SecretAccessKey: "secret", SessionToken: "token"}, creds)

		a := langx.Clone(Agent{}, opt)
		require.Empty(t, a.volumes, "the runner installs the credentials, nothing is mounted")
	})

	t.Run("should fail when there are no credentials", func(t *testing.T) {
		isolate(t)

		_, err := AgentOptionAWSCredentials(t.Context(), envx.Build())
		require.Error(t, err)
	})
}
