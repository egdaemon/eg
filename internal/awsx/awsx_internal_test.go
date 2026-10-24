package awsx

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/generations"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

const response = `{"id_token":"idtoken","role_arn":"arn:aws:iam::123456789012:role/example","region":"us-east-2"}`

func TestWanted(t *testing.T) {
	envfile := func(t *testing.T, content string) string {
		p := filepath.Join(t.TempDir(), eg.EnvironFile)
		require.NoError(t, os.WriteFile(p, []byte(content), 0600))
		return p
	}

	t.Run("should not be wanted by default", func(t *testing.T) {
		require.False(t, wanted(envfile(t, "FOO=bar\n")))
		require.False(t, wanted(filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("should not be wanted when disabled or declaring only the gcp enablement", func(t *testing.T) {
		require.False(t, wanted(envfile(t, "EG_AWS_WORKLOAD_IDENTITY_ENABLED=false\n")))
		require.False(t, wanted(envfile(t, "EG_GCP_WORKLOAD_IDENTITY_ENABLED=true\n")))
	})

	t.Run("should be wanted when the workload enabled it", func(t *testing.T) {
		require.True(t, wanted(envfile(t, "FOO=bar\nEG_AWS_WORKLOAD_IDENTITY_ENABLED=true\n")))
	})
}

func TestProvided(t *testing.T) {
	envfile := func(t *testing.T, content string) string {
		p := filepath.Join(t.TempDir(), eg.EnvironFile)
		require.NoError(t, os.WriteFile(p, []byte(content), 0600))
		return p
	}

	t.Run("should not be provided by default", func(t *testing.T) {
		require.False(t, provided(envfile(t, "FOO=bar\n")))
		require.False(t, provided(filepath.Join(t.TempDir(), "missing")))
		require.False(t, provided(envfile(t, "EG_UNSAFE_AWS_CREDENTIALS_B64=\n")))
	})

	t.Run("should be provided when the agent set the credentials", func(t *testing.T) {
		require.True(t, provided(envfile(t, "FOO=bar\nEG_UNSAFE_AWS_CREDENTIALS_B64=e30=\n")))
	})
}

func TestSettings(t *testing.T) {
	t.Run("should be empty without a role or credentials", func(t *testing.T) {
		require.Empty(t, Settings{}.Environ())
	})

	t.Run("should describe the web identity", func(t *testing.T) {
		require.ElementsMatch(t, []string{
			"AWS_ROLE_ARN=arn:aws:iam::123456789012:role/example",
			"AWS_WEB_IDENTITY_TOKEN_FILE=/token",
			"AWS_ROLE_SESSION_NAME=eg-1",
			"AWS_REGION=us-east-2",
		}, Settings{RoleARN: "arn:aws:iam::123456789012:role/example", TokenFile: "/token", SessionName: "eg-1", Region: "us-east-2"}.Environ())
	})

	t.Run("should describe the credentials file", func(t *testing.T) {
		require.ElementsMatch(t, []string{"AWS_SHARED_CREDENTIALS_FILE=/credentials"}, Settings{CredentialsFile: "/credentials"}.Environ())
	})
}

func TestInstall(t *testing.T) {
	nochown := func(string) error { return nil }

	issued := func(t *testing.T, content string) string {
		t.Helper()
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, responseFilename), []byte(content), 0600))
		return src
	}

	read := func(t *testing.T, path string) string {
		t.Helper()
		encoded, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(encoded)
	}

	t.Run("should install nothing when no credentials were provided", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "aws")
		s, err := install(t.Context(), dst, filepath.Join(t.TempDir(), "missing"), nil, "session", time.Hour, nochown)
		require.NoError(t, err)
		require.Empty(t, s.Environ())
		require.NoDirExists(t, dst)
	})

	t.Run("should install the credentials provided by the agent", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "aws")
		provided := []byte(`{"access_key_id":"akid","secret_access_key":"secret","session_token":"token"}`)
		s, err := install(t.Context(), dst, filepath.Join(t.TempDir(), "missing"), provided, "session", time.Hour, nochown)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dst, generationsDirectory, generations.Current, credentialsFilename), s.CredentialsFile)
		require.Empty(t, s.RoleARN)
		require.Equal(t, "[default]\naws_access_key_id = akid\naws_secret_access_key = secret\naws_session_token = token\n", read(t, s.CredentialsFile))
	})

	t.Run("should reject credentials provided by the agent that are not complete", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "aws")
		_, err := install(t.Context(), dst, filepath.Join(t.TempDir(), "missing"), []byte(`{"access_key_id":"akid"}`), "session", time.Hour, nochown)
		require.Error(t, err)
	})

	t.Run("should install the web identity issued by the runner", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "aws")
		s, err := install(t.Context(), dst, issued(t, response), nil, "eg-1", time.Hour, nochown)
		require.NoError(t, err)
		require.Equal(t, "arn:aws:iam::123456789012:role/example", s.RoleARN)
		require.Equal(t, "us-east-2", s.Region)
		require.Equal(t, "eg-1", s.SessionName)

		// the token is read through current, which follows every new generation.
		require.Equal(t, filepath.Join(dst, generationsDirectory, generations.Current, tokenFilename), s.TokenFile)
		require.Equal(t, "idtoken", read(t, s.TokenFile))
	})

	t.Run("should install nothing when the runner issued no credentials", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "aws")
		s, err := install(t.Context(), dst, issued(t, `{}`), nil, "eg-1", time.Hour, nochown)
		require.NoError(t, err)
		require.Empty(t, s.Environ())
		require.NoDirExists(t, filepath.Join(dst, generationsDirectory, generations.Current))
	})

	t.Run("should leave the existing configuration untouched", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "aws")
		config := filepath.Join(dst, "config")
		require.NoError(t, os.MkdirAll(dst, 0770))
		require.NoError(t, os.WriteFile(config, []byte("[default]\nregion = eu-west-1\n"), 0600))

		_, err := install(t.Context(), dst, issued(t, response), nil, "eg-1", time.Hour, nochown)
		require.NoError(t, err)
		require.Equal(t, "[default]\nregion = eu-west-1\n", read(t, config))
	})

	t.Run("should follow the credentials fetched by the runner", func(t *testing.T) {
		ctx, done := context.WithCancel(t.Context())
		root, err := os.MkdirTemp("", "awsx")
		require.NoError(t, err)
		t.Cleanup(func() {
			// the refresh stops after the context is done, but may be publishing a final generation.
			done()
			require.Eventually(t, func() bool { return os.RemoveAll(root) == nil }, 5*time.Second, 10*time.Millisecond)
		})

		dst := filepath.Join(root, "aws")
		src := issued(t, response)
		s, err := install(ctx, dst, src, nil, "eg-1", 10*time.Millisecond, nochown)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(src, responseFilename), []byte(`{"id_token":"refreshed","role_arn":"arn:aws:iam::123456789012:role/example"}`), 0600))
		require.Eventually(t, func() bool {
			encoded, err := os.ReadFile(s.TokenFile)
			return err == nil && string(encoded) == "refreshed"
		}, 5*time.Second, 10*time.Millisecond)
	})
}

func TestProvision(t *testing.T) {
	enabled := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, eg.EnvironFile), []byte("EG_AWS_WORKLOAD_IDENTITY_ENABLED=true\n"), 0600))
		return dir
	}

	fake := func(response string, err error, calls *int) fetch {
		return func(ctx context.Context, c *http.Client, dst, token string) error {
			*calls++
			require.Equal(t, "token", token)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dst, responseFilename), []byte(response), 0600)
		}
	}

	t.Run("should not fetch when the workload did not ask", func(t *testing.T) {
		var calls int
		s, err := provision(t.Context(), fake("", nil, &calls), nil, t.TempDir(), "token")
		require.NoError(t, err)
		require.Empty(t, s.Environ())
		require.Zero(t, calls)
	})

	t.Run("should provision the web identity into the aws directory when enabled", func(t *testing.T) {
		var calls int
		dir := enabled(t)
		s, err := provision(t.Context(), fake(response, nil, &calls), nil, dir, "token")
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, "arn:aws:iam::123456789012:role/example", s.RoleARN)
		require.Equal(t, filepath.Join(dir, Directory, tokenFilename), s.TokenFile)
		encoded, err := os.ReadFile(s.TokenFile)
		require.NoError(t, err)
		require.Equal(t, "idtoken", string(encoded))
	})

	t.Run("should not fetch when the agent provided the credentials", func(t *testing.T) {
		var calls int
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, eg.EnvironFile), []byte("EG_AWS_WORKLOAD_IDENTITY_ENABLED=true\nEG_UNSAFE_AWS_CREDENTIALS_B64=e30=\n"), 0600))
		s, err := provision(t.Context(), fake(response, nil, &calls), nil, dir, "token")
		require.NoError(t, err)
		require.Empty(t, s.Environ())
		require.Zero(t, calls)
	})

	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		t.Run("should run without credentials when the account has no connection", func(t *testing.T) {
			var calls int
			dir := enabled(t)
			_, cause := httpx.AsError(&http.Response{StatusCode: code, Status: http.StatusText(code)}, nil)
			s, err := provision(t.Context(), fake("", cause, &calls), nil, dir, "token")
			require.NoError(t, err)
			require.Empty(t, s.Environ())
			require.NoDirExists(t, filepath.Join(dir, Directory))
		})
	}

	t.Run("should fail on other errors", func(t *testing.T) {
		var calls int
		_, cause := httpx.AsError(&http.Response{StatusCode: http.StatusInternalServerError, Status: "500"}, nil)
		_, err := provision(t.Context(), fake("", cause, &calls), nil, enabled(t), "token")
		require.Error(t, err)
	})
}
