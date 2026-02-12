package shell_test

import (
	"fmt"
	"os/user"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestRun0Commands(t *testing.T) {
	u := userx.CurrentUserOrDefault(user.User{Username: "egd"})
	defaultuser := u.Username

	type example struct {
		description string
		cmd         shell.Command
		expected    string
	}
	examples := []example{
		{
			description: "custom user and group",
			cmd:         shell.New("echo \"hello world\"").User("derp").Group("derp").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    "::run0:--user=derp --group=derp bash -c echo \"hello world\"",
		},
		{
			description: "default user and group",
			cmd:         shell.New("psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    fmt.Sprintf("::run0:--user=%s --group=%s bash -c psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"", defaultuser, defaultuser),
		},
		{
			description: "single environment variable",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("GH_TOKEN", "secret123").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    ":GH_TOKEN=secret123:run0:--user=egd --group=egd --setenv=GH_TOKEN bash -c echo hello",
		},
		{
			description: "multiple environment variables",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("FOO", "bar").Environ("BAZ", "qux").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    ":FOO=bar:BAZ=qux:run0:--user=egd --group=egd --setenv=FOO --setenv=BAZ bash -c echo hello",
		},
		{
			description: "environ from slice",
			cmd:         shell.New("echo hello").User("egd").Group("egd").EnvironFrom("KEY1=val1", "KEY2=val2").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    ":KEY1=val1:KEY2=val2:run0:--user=egd --group=egd --setenv=KEY1 --setenv=KEY2 bash -c echo hello",
		},
		{
			description: "environ with integer value",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("PORT", 5432).UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    ":PORT=5432:run0:--user=egd --group=egd --setenv=PORT bash -c echo hello",
		},
		{
			description: "environ with empty string value",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("PAGER", "").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    ":PAGER=:run0:--user=egd --group=egd --setenv=PAGER bash -c echo hello",
		},
		{
			description: "as sets both user and group",
			cmd:         shell.New("pg_isready").As("postgres").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    "::run0:--user=postgres --group=postgres bash -c pg_isready",
		},
		{
			description: "privileged runs as root",
			cmd:         shell.New("apt-get update").Privileged().UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    "::run0:--user=root --group=root bash -c apt-get update",
		},
		{
			description: "directory is passed through",
			cmd:         shell.New("ls -lha").User("egd").Group("egd").Directory("/workspace").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    "/workspace::run0:--user=egd --group=egd bash -c ls -lha",
		},
		{
			description: "directory with environment variables",
			cmd:         shell.New("make build").User("egd").Group("egd").Directory("/workspace").Environ("CC", "gcc").Environ("CFLAGS", "-O2").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    "/workspace:CC=gcc:CFLAGS=-O2:run0:--user=egd --group=egd --setenv=CC --setenv=CFLAGS bash -c make build",
		},
		{
			description: "privileged with environment variables",
			cmd:         shell.New("systemctl restart nginx").Privileged().Environ("SYSTEMD_LOG_LEVEL", "debug").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    ":SYSTEMD_LOG_LEVEL=debug:run0:--user=root --group=root --setenv=SYSTEMD_LOG_LEVEL bash -c systemctl restart nginx",
		},
		{
			description: "full combination: as + directory + multiple environ",
			cmd:         shell.New("psql -c \"SELECT 1\"").As("postgres").Directory("/tmp").Environ("PAGER", "").Environ("PGPASSWORD", "secret").UnsafeEntrypoint(shell.UnsafeRun0Entry),
			expected:    "/tmp:PAGER=:PGPASSWORD=secret:run0:--user=postgres --group=postgres --setenv=PAGER --setenv=PGPASSWORD bash -c psql -c \"SELECT 1\"",
		},
	}

	ctx, done := testx.Context(t)
	defer done()

	for _, e := range examples {
		t.Run(e.description, func(t *testing.T) {
			rec := shell.NewRecorder(&e.cmd)
			require.NoError(t, shell.Run(ctx, e.cmd))
			require.Equal(t, e.expected, rec.Result())
		})
	}

	t.Run("environ carries through to derived commands", func(t *testing.T) {
		runtime := shell.Runtime().UnsafeEntrypoint(shell.UnsafeRun0Entry).As("postgres").Environ("PAGER", "").Environ("PGPASSWORD", "secret")
		cmd := runtime.New("psql --version")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(ctx, cmd))
		require.Equal(t, ":PAGER=:PGPASSWORD=secret:run0:--user=postgres --group=postgres --setenv=PAGER --setenv=PGPASSWORD bash -c psql --version", rec.Result())
	})

	t.Run("derived commands get independent environ copies", func(t *testing.T) {
		runtime := shell.Runtime().UnsafeEntrypoint(shell.UnsafeRun0Entry).As("deploy").Environ("APP_ENV", "production")
		cmd1 := runtime.New("echo first").Environ("EXTRA", "one")
		cmd2 := runtime.New("echo second").Environ("EXTRA", "two")

		rec1 := shell.NewRecorder(&cmd1)
		require.NoError(t, shell.Run(ctx, cmd1))
		require.Equal(t, ":APP_ENV=production:EXTRA=one:run0:--user=deploy --group=deploy --setenv=APP_ENV --setenv=EXTRA bash -c echo first", rec1.Result())

		rec2 := shell.NewRecorder(&cmd2)
		require.NoError(t, shell.Run(ctx, cmd2))
		require.Equal(t, ":APP_ENV=production:EXTRA=two:run0:--user=deploy --group=deploy --setenv=APP_ENV --setenv=EXTRA bash -c echo second", rec2.Result())
	})

	t.Run("derived command does not mutate template environ", func(t *testing.T) {
		runtime := shell.Runtime().UnsafeEntrypoint(shell.UnsafeRun0Entry).As("egd").Environ("BASE", "value")

		// Create a derived command that adds an env var
		_ = runtime.New("echo derived").Environ("ADDED", "extra")

		// Create another derived command from the original template
		cmd := runtime.New("echo original")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(ctx, cmd))
		require.Equal(t, ":BASE=value:run0:--user=egd --group=egd --setenv=BASE bash -c echo original", rec.Result())
	})

	t.Run("multiple commands record sequentially", func(t *testing.T) {
		runtime := shell.Runtime().UnsafeEntrypoint(shell.UnsafeRun0Entry).As("postgres").Environ("PAGER", "")
		cmd1 := runtime.New("pg_isready")
		cmd2 := runtime.New("psql -c \"SELECT 1\"")

		rec1 := shell.NewRecorder(&cmd1)
		rec2 := shell.NewRecorder(&cmd2)
		require.NoError(t, shell.Run(ctx, cmd1, cmd2))
		require.Equal(t, ":PAGER=:run0:--user=postgres --group=postgres --setenv=PAGER bash -c pg_isready", rec1.Result())
		require.Equal(t, ":PAGER=:run0:--user=postgres --group=postgres --setenv=PAGER bash -c psql -c \"SELECT 1\"", rec2.Result())
	})
}
