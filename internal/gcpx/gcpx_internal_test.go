package gcpx

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

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

	t.Run("should not be wanted when disabled or declaring only the google credentials", func(t *testing.T) {
		require.False(t, wanted(envfile(t, "EG_GCP_WORKLOAD_IDENTITY_ENABLED=false\n")))
		require.False(t, wanted(envfile(t, "GOOGLE_APPLICATION_CREDENTIALS=/anything\n")))
	})

	t.Run("should be wanted when the workload enabled it", func(t *testing.T) {
		require.True(t, wanted(envfile(t, "FOO=bar\nEG_GCP_WORKLOAD_IDENTITY_ENABLED=true\n")))
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
		require.False(t, provided(envfile(t, "EG_UNSAFE_GOOGLE_APPLICATION_CREDENTIALS_B64=\n")))
	})

	t.Run("should be provided when the agent set the credentials", func(t *testing.T) {
		require.True(t, provided(envfile(t, "FOO=bar\nEG_UNSAFE_GOOGLE_APPLICATION_CREDENTIALS_B64=e30=\n")))
	})
}

func TestInstall(t *testing.T) {
	nochown := func(string) error { return nil }

	const response = `{"id_token":"idtoken","audience":"//iam.googleapis.com/example"}`

	// a runner credentials directory holding the fetched response.
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

	generations := func(t *testing.T, base string) []string {
		t.Helper()
		matches, err := filepath.Glob(filepath.Join(base, pastGenerations, "*"))
		require.NoError(t, err)
		return matches
	}

	t.Run("should install nothing when no credentials were provided", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "gcloud")
		adc, err := install(t.Context(), dst, filepath.Join(t.TempDir(), "missing"), nil, time.Hour, nochown)
		require.NoError(t, err)
		require.Empty(t, adc)
		require.NoDirExists(t, dst)
	})

	t.Run("should install the credentials provided by the agent", func(t *testing.T) {
		var chowned []string
		dst := filepath.Join(t.TempDir(), "gcloud")
		adc, err := install(t.Context(), dst, filepath.Join(t.TempDir(), "missing"), []byte(`{"type":"authorized_user"}`), time.Hour, func(path string) error {
			chowned = append(chowned, path)
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dst, adcInstalledFilename), adc)
		require.Equal(t, `{"type":"authorized_user"}`, read(t, adc))
		require.Contains(t, chowned, filepath.Join(dst, generationsDirectory))
		require.Len(t, generations(t, filepath.Join(dst, generationsDirectory)), 1)
	})

	t.Run("should install the credentials issued by the runner", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "gcloud")
		adc, err := install(t.Context(), dst, issued(t, response), nil, time.Hour, nochown)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dst, adcInstalledFilename), adc)

		// the identity token is read through current, which follows every new generation.
		var cfg externalAccountConfig
		require.NoError(t, json.Unmarshal([]byte(read(t, adc)), &cfg))
		require.Equal(t, filepath.Join(dst, generationsDirectory, currentGeneration, idTokenFilename), cfg.CredentialSource.File)
		require.Equal(t, "idtoken", read(t, cfg.CredentialSource.File))
	})

	t.Run("should install nothing when the runner issued no credentials", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "gcloud")
		adc, err := install(t.Context(), dst, issued(t, `{}`), nil, time.Hour, nochown)
		require.NoError(t, err)
		require.Empty(t, adc)
		require.NoFileExists(t, filepath.Join(dst, adcInstalledFilename))
		require.Empty(t, generations(t, filepath.Join(dst, generationsDirectory)))
	})

	t.Run("should leave the existing configuration untouched", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "gcloud")
		config := filepath.Join(dst, "configurations", "config_default")
		require.NoError(t, os.MkdirAll(filepath.Dir(config), 0770))
		require.NoError(t, os.WriteFile(config, []byte("[core]\npass_credentials_to_gsutil = true\n"), 0600))

		_, err := install(t.Context(), dst, issued(t, response), nil, time.Hour, nochown)
		require.NoError(t, err)
		require.Equal(t, "[core]\npass_credentials_to_gsutil = true\n", read(t, config))
	})

	t.Run("should follow the credentials fetched by the runner", func(t *testing.T) {
		ctx, done := context.WithCancel(t.Context())
		root, err := os.MkdirTemp("", "gcpx")
		require.NoError(t, err)
		t.Cleanup(func() {
			// the refresh stops after the context is done, but may be publishing a final generation.
			done()
			require.Eventually(t, func() bool { return os.RemoveAll(root) == nil }, 5*time.Second, 10*time.Millisecond)
		})

		dst := filepath.Join(root, "gcloud")
		src := issued(t, response)
		_, err = install(ctx, dst, src, nil, 10*time.Millisecond, nochown)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(src, responseFilename), []byte(`{"id_token":"refreshed","audience":"//iam.googleapis.com/example"}`), 0600))
		require.Eventually(t, func() bool {
			encoded, err := os.ReadFile(filepath.Join(dst, generationsDirectory, currentGeneration, idTokenFilename))
			return err == nil && string(encoded) == "refreshed"
		}, 5*time.Second, 10*time.Millisecond)
	})
}

func TestPublish(t *testing.T) {
	nochown := func(string) error { return nil }

	write := func(content string) func(string) (bool, error) {
		return func(dir string) (bool, error) {
			return true, os.WriteFile(filepath.Join(dir, "file"), []byte(content), 0600)
		}
	}

	t.Run("should make the generation current", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		ok, err := publish(t.Context(), base, nochown, write("first"))
		require.NoError(t, err)
		require.True(t, ok)

		encoded, err := os.ReadFile(filepath.Join(base, currentGeneration, "file"))
		require.NoError(t, err)
		require.Equal(t, "first", string(encoded))
	})

	t.Run("should publish nothing when there is nothing to build", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		ok, err := publish(t.Context(), base, nochown, func(string) (bool, error) { return false, nil })
		require.NoError(t, err)
		require.False(t, ok)
		require.NoFileExists(t, filepath.Join(base, currentGeneration))

		entries, err := os.ReadDir(filepath.Join(base, pastGenerations))
		require.NoError(t, err)
		require.Empty(t, entries, "expected the unpublished generation to be removed")
	})

	t.Run("should keep the previous generation and remove older ones", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		for _, content := range []string{"a", "b", "c", "d"} {
			_, err := publish(t.Context(), base, nochown, write(content))
			require.NoError(t, err)
		}

		matches, err := filepath.Glob(filepath.Join(base, pastGenerations, "*"))
		require.NoError(t, err)
		require.Len(t, matches, 2)

		encoded, err := os.ReadFile(filepath.Join(base, currentGeneration, "file"))
		require.NoError(t, err)
		require.Equal(t, "d", string(encoded))
	})

	t.Run("should never expose a partial generation", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		_, err := publish(t.Context(), base, nochown, write(strings.Repeat("a", 1<<16)))
		require.NoError(t, err)

		done := make(chan struct{})
		var torn atomic.Bool
		var readers sync.WaitGroup
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				encoded, err := os.ReadFile(filepath.Join(base, currentGeneration, "file"))
				if err != nil || len(encoded) != 1<<16 || len(strings.Trim(string(encoded), string(encoded[:1]))) != 0 {
					torn.Store(true)
				}
			}
		}()

		for i := 0; i < 50; i++ {
			_, err := publish(t.Context(), base, nochown, write(strings.Repeat(string(rune('a'+i%26)), 1<<16)))
			require.NoError(t, err)
		}

		close(done)
		readers.Wait()
		require.False(t, torn.Load())
	})
}

func TestProvision(t *testing.T) {
	// a runtime directory of a workload that enabled gcp workload identity.
	enabled := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, eg.EnvironFile), []byte("EG_GCP_WORKLOAD_IDENTITY_ENABLED=true\n"), 0600))
		return dir
	}

	const credentials = `{"id_token":"idtoken","audience":"//iam.googleapis.com/example"}`

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
		adc, err := provision(t.Context(), fake("", nil, &calls), nil, t.TempDir(), "token")
		require.NoError(t, err)
		require.Empty(t, adc)
		require.Zero(t, calls)
	})

	t.Run("should provision the credentials into the gcloud directory when enabled", func(t *testing.T) {
		var calls int
		dir := enabled(t)
		adc, err := provision(t.Context(), fake(credentials, nil, &calls), nil, dir, "token")
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, filepath.Join(dir, Directory, adcFilename), adc)
		require.FileExists(t, adc)
		require.FileExists(t, filepath.Join(dir, Directory, idTokenFilename))
	})

	t.Run("should not fetch when the agent provided the credentials", func(t *testing.T) {
		var calls int
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, eg.EnvironFile), []byte("EG_GCP_WORKLOAD_IDENTITY_ENABLED=true\nEG_UNSAFE_GOOGLE_APPLICATION_CREDENTIALS_B64=e30=\n"), 0600))
		adc, err := provision(t.Context(), fake(credentials, nil, &calls), nil, dir, "token")
		require.NoError(t, err)
		require.Empty(t, adc)
		require.Zero(t, calls)
	})

	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		t.Run("should run without credentials when the account has no connection", func(t *testing.T) {
			var calls int
			dir := enabled(t)
			_, cause := httpx.AsError(&http.Response{StatusCode: code, Status: http.StatusText(code)}, nil)
			adc, err := provision(t.Context(), fake("", cause, &calls), nil, dir, "token")
			require.NoError(t, err)
			require.Empty(t, adc)
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
