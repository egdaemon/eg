package main

import (
	"os"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestDefaultGcloudCredentials(t *testing.T) {
	t.Run("should point gcloud at the installed credentials", func(t *testing.T) {
		t.Setenv(eg.EnvCloudSDKCredentialFileOverride, "")
		require.NoError(t, os.Unsetenv(eg.EnvCloudSDKCredentialFileOverride))

		require.NoError(t, defaultgcloudcredentials("/home/egd/.config/gcloud/application_default_credentials.json"))
		require.Equal(t, "/home/egd/.config/gcloud/application_default_credentials.json", os.Getenv(eg.EnvCloudSDKCredentialFileOverride))
	})

	t.Run("should not set anything when no credentials were installed", func(t *testing.T) {
		t.Setenv(eg.EnvCloudSDKCredentialFileOverride, "")
		require.NoError(t, os.Unsetenv(eg.EnvCloudSDKCredentialFileOverride))

		require.NoError(t, defaultgcloudcredentials(""))
		_, set := os.LookupEnv(eg.EnvCloudSDKCredentialFileOverride)
		require.False(t, set)
	})

	t.Run("should keep the override of the user", func(t *testing.T) {
		t.Setenv(eg.EnvCloudSDKCredentialFileOverride, "/user/adc.json")

		require.NoError(t, defaultgcloudcredentials("/home/egd/.config/gcloud/application_default_credentials.json"))
		require.Equal(t, "/user/adc.json", os.Getenv(eg.EnvCloudSDKCredentialFileOverride))
	})
}
