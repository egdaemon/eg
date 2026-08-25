package eggpg_test

import (
	"strings"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/eggpg"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	// value returns the value for the first entry in env prefixed "key=", and
	// whether it was found.
	value := func(env []string, key string) (string, bool) {
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, key+"="); ok {
				return v, true
			}
		}
		return "", false
	}

	t.Run("returns the configured identity and default home", func(t *testing.T) {
		t.Setenv(eggpg.EnvHome, "")

		got := eggpg.Env(eggpg.Options().
			Name("Test User").
			Email("test@example.com").
			Seed("test-seed")...)

		home, ok := value(got, eggpg.EnvHome)
		require.True(t, ok)
		require.Equal(t, "/home/egd/.gnupg", home)

		name, ok := value(got, eggpg.EnvName)
		require.True(t, ok)
		require.Equal(t, "Test User", name)

		email, ok := value(got, eggpg.EnvEmail)
		require.True(t, ok)
		require.Equal(t, "test@example.com", email)

		seed, ok := value(got, eggpg.EnvSeed)
		require.True(t, ok)
		require.Equal(t, "test-seed", seed)

		keyringhome, ok := value(got, "EG_GPG_KEYRING_HOME")
		require.True(t, ok)
		require.NotEmpty(t, keyringhome)
	})

	t.Run("honors an explicit Home when it matches no override condition", func(t *testing.T) {
		got := eggpg.Env(eggpg.Options().
			Name("Test User").
			Email("test@example.com").
			Seed("test-seed").
			Home("/custom/gnupg")...)

		home, ok := value(got, eggpg.EnvHome)
		require.True(t, ok)
		require.Equal(t, "/custom/gnupg", home)
	})

	t.Run("same seed produces the same EG_GPG_KEYRING_HOME across calls", func(t *testing.T) {
		opts := eggpg.Options().
			Name("Test User").
			Email("test@example.com").
			Seed("stable-seed")

		got1 := eggpg.Env(opts...)
		got2 := eggpg.Env(opts...)

		home1, ok := value(got1, "EG_GPG_KEYRING_HOME")
		require.True(t, ok)
		home2, ok := value(got2, "EG_GPG_KEYRING_HOME")
		require.True(t, ok)
		require.Equal(t, home1, home2)
	})

	t.Run("IgnoreLocalGNU does not bypass when the resolved home is already the default", func(t *testing.T) {
		t.Setenv(eggpg.EnvHome, "")

		got := eggpg.Env(eggpg.Options().
			Name("Test User").
			Email("test@example.com").
			Seed("test-seed").
			IgnoreLocalGNU()...)

		name, ok := value(got, eggpg.EnvName)
		require.True(t, ok)
		require.Equal(t, "Test User", name)
	})

	t.Run("IgnoreLocalGNU falls back to the safe default env when home is overridden", func(t *testing.T) {
		t.Setenv(eggpg.EnvHome, "")
		t.Setenv(eggpg.EnvName, "Fallback User")
		t.Setenv(eggpg.EnvEmail, "fallback@example.com")
		t.Setenv(eggpg.EnvSeed, "fallback-seed")

		got := eggpg.Env(eggpg.Options().
			Name("Custom User").
			Email("custom@example.com").
			Seed("custom-seed").
			Home("/some/other/path").
			IgnoreLocalGNU()...)

		home, ok := value(got, eggpg.EnvHome)
		require.True(t, ok)
		require.Equal(t, "/home/egd/.gnupg", home, "should redirect away from the overridden home")

		// the fallback re-derives from the ambient environment rather than the
		// identity passed to Options(), so it reflects the env-var values, not
		// "Custom User"/"custom@example.com"/"custom-seed".
		name, ok := value(got, eggpg.EnvName)
		require.True(t, ok)
		require.Equal(t, "Fallback User", name)
	})

	t.Run("IgnoreLocalGNU fallback reflects the ambient GNUPGHOME, not a hardcoded default", func(t *testing.T) {
		t.Setenv(eggpg.EnvHome, "/ambient/gnupg/home")
		t.Setenv(eggpg.EnvName, "Fallback User")
		t.Setenv(eggpg.EnvEmail, "fallback@example.com")
		t.Setenv(eggpg.EnvSeed, "fallback-seed")

		got := eggpg.Env(eggpg.Options().
			Name("Custom User").
			Email("custom@example.com").
			Seed("custom-seed").
			Home("/explicit/override/path").
			IgnoreLocalGNU()...)

		home, ok := value(got, eggpg.EnvHome)
		require.True(t, ok)
		require.Equal(t, "/ambient/gnupg/home", home,
			"fallback should re-read the current ambient GNUPGHOME via Options(), not the overridden opts.home and not a hardcoded default")
	})

	t.Run("IgnoreLocalGNU fallback degrades gracefully if the ambient environment lacks identity vars", func(t *testing.T) {
		t.Setenv(eggpg.EnvHome, "")
		t.Setenv(eggpg.EnvName, "")
		t.Setenv(eggpg.EnvEmail, "")
		t.Setenv(eggpg.EnvSeed, "")

		var got []string
		got = eggpg.Env(eggpg.Options().
			Name("Custom User").
			Email("custom@example.com").
			Seed("custom-seed").
			Home("/some/other/path").
			IgnoreLocalGNU()...)

		home, ok := value(got, eggpg.EnvHome)
		require.True(t, ok)
		require.Equal(t, "/home/egd/.gnupg", home, "should still redirect away from the overridden home")

		// missing ambient identity vars come through empty rather than causing
		// a panic or an error.
		name, ok := value(got, eggpg.EnvName)
		require.True(t, ok)
		require.Empty(t, name)
	})
}
