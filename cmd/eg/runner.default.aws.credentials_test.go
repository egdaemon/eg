package main

import (
	"os"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/awsx"
	"github.com/stretchr/testify/require"
)

func TestDefaultAWSCredentials(t *testing.T) {
	keys := []string{eg.EnvAWSRoleARN, eg.EnvAWSWebIdentityTokenFile, eg.EnvAWSRoleSessionName, eg.EnvAWSRegion, "AWS_ACCESS_KEY_ID", "AWS_PROFILE", "AWS_SHARED_CREDENTIALS_FILE"}

	// start from an environment without any aws settings.
	clean := func(t *testing.T) {
		for _, k := range keys {
			t.Setenv(k, "")
			require.NoError(t, os.Unsetenv(k))
		}
	}

	settings := awsx.Settings{RoleARN: "arn:aws:iam::123456789012:role/example", TokenFile: "/token", SessionName: "eg-1", Region: "us-east-2"}

	t.Run("should point the aws sdks at the installed credentials", func(t *testing.T) {
		clean(t)

		require.NoError(t, defaultawscredentials(settings))
		require.Equal(t, "arn:aws:iam::123456789012:role/example", os.Getenv(eg.EnvAWSRoleARN))
		require.Equal(t, "/token", os.Getenv(eg.EnvAWSWebIdentityTokenFile))
		require.Equal(t, "eg-1", os.Getenv(eg.EnvAWSRoleSessionName))
		require.Equal(t, "us-east-2", os.Getenv(eg.EnvAWSRegion))
	})

	t.Run("should not set anything when no credentials were installed", func(t *testing.T) {
		clean(t)

		require.NoError(t, defaultawscredentials(awsx.Settings{}))
		for _, k := range keys {
			_, set := os.LookupEnv(k)
			require.False(t, set, k)
		}
	})

	for _, k := range []string{eg.EnvAWSRoleARN, "AWS_ACCESS_KEY_ID", "AWS_PROFILE"} {
		t.Run("should keep the credentials of the user when "+k+" is set", func(t *testing.T) {
			clean(t)
			t.Setenv(k, "user")

			require.NoError(t, defaultawscredentials(settings))
			require.Equal(t, "user", os.Getenv(k))
			_, set := os.LookupEnv(eg.EnvAWSWebIdentityTokenFile)
			require.False(t, set, "mixing the role of the user with the token of the workload would not work")
		})
	}
}
