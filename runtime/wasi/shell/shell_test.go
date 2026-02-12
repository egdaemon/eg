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

func TestShellCommands(t *testing.T) {
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
			cmd:         shell.New("echo \"hello world\"").User("derp").Group("derp"),
			expected:    "::sudo:-H -u derp -g derp bash -c echo \"hello world\"",
		},
		{
			description: "default user and group",
			cmd:         shell.New("psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\""),
			expected:    fmt.Sprintf("::sudo:-H -u %s -g %s bash -c psql --no-psqlrc -U postgres -d postgres -q -At -c \"SELECT pg_reload_conf();\"", defaultuser, defaultuser),
		},
		{
			description: "single environment variable",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("GH_TOKEN", "secret123"),
			expected:    ":GH_TOKEN=secret123:sudo:-H -u egd -g egd env GH_TOKEN=secret123 bash -c echo hello",
		},
		{
			description: "multiple environment variables",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("FOO", "bar").Environ("BAZ", "qux"),
			expected:    ":FOO=bar:BAZ=qux:sudo:-H -u egd -g egd env FOO=bar BAZ=qux bash -c echo hello",
		},
		{
			description: "environ from slice",
			cmd:         shell.New("echo hello").User("egd").Group("egd").EnvironFrom("KEY1=val1", "KEY2=val2"),
			expected:    ":KEY1=val1:KEY2=val2:sudo:-H -u egd -g egd env KEY1=val1 KEY2=val2 bash -c echo hello",
		},
		{
			description: "environ with integer value",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("PORT", 5432),
			expected:    ":PORT=5432:sudo:-H -u egd -g egd env PORT=5432 bash -c echo hello",
		},
		{
			description: "environ with empty string value",
			cmd:         shell.New("echo hello").User("egd").Group("egd").Environ("PAGER", ""),
			expected:    ":PAGER=:sudo:-H -u egd -g egd env PAGER= bash -c echo hello",
		},
		{
			description: "as sets both user and group",
			cmd:         shell.New("pg_isready").As("postgres"),
			expected:    "::sudo:-H -u postgres -g postgres bash -c pg_isready",
		},
		{
			description: "privileged runs as root",
			cmd:         shell.New("apt-get update").Privileged(),
			expected:    "::sudo:-H -u root -g root bash -c apt-get update",
		},
		{
			description: "directory is passed through",
			cmd:         shell.New("ls -lha").User("egd").Group("egd").Directory("/workspace"),
			expected:    "/workspace::sudo:-H -u egd -g egd bash -c ls -lha",
		},
		{
			description: "directory with environment variables",
			cmd:         shell.New("make build").User("egd").Group("egd").Directory("/workspace").Environ("CC", "gcc").Environ("CFLAGS", "-O2"),
			expected:    "/workspace:CC=gcc:CFLAGS=-O2:sudo:-H -u egd -g egd env CC=gcc CFLAGS=-O2 bash -c make build",
		},
		{
			description: "privileged with environment variables",
			cmd:         shell.New("systemctl restart nginx").Privileged().Environ("SYSTEMD_LOG_LEVEL", "debug"),
			expected:    ":SYSTEMD_LOG_LEVEL=debug:sudo:-H -u root -g root env SYSTEMD_LOG_LEVEL=debug bash -c systemctl restart nginx",
		},
		{
			description: "full combination: as + directory + multiple environ",
			cmd:         shell.New("psql -c \"SELECT 1\"").As("postgres").Directory("/tmp").Environ("PAGER", "").Environ("PGPASSWORD", "secret"),
			expected:    "/tmp:PAGER=:PGPASSWORD=secret:sudo:-H -u postgres -g postgres env PAGER= PGPASSWORD=secret bash -c psql -c \"SELECT 1\"",
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
		runtime := shell.Runtime().As("postgres").Environ("PAGER", "").Environ("PGPASSWORD", "secret")
		cmd := runtime.New("psql --version")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(ctx, cmd))
		require.Equal(t, ":PAGER=:PGPASSWORD=secret:sudo:-H -u postgres -g postgres env PAGER= PGPASSWORD=secret bash -c psql --version", rec.Result())
	})

	t.Run("derived commands get independent environ copies", func(t *testing.T) {
		runtime := shell.Runtime().As("deploy").Environ("APP_ENV", "production")
		cmd1 := runtime.New("echo first").Environ("EXTRA", "one")
		cmd2 := runtime.New("echo second").Environ("EXTRA", "two")

		rec1 := shell.NewRecorder(&cmd1)
		require.NoError(t, shell.Run(ctx, cmd1))
		require.Equal(t, ":APP_ENV=production:EXTRA=one:sudo:-H -u deploy -g deploy env APP_ENV=production EXTRA=one bash -c echo first", rec1.Result())

		rec2 := shell.NewRecorder(&cmd2)
		require.NoError(t, shell.Run(ctx, cmd2))
		require.Equal(t, ":APP_ENV=production:EXTRA=two:sudo:-H -u deploy -g deploy env APP_ENV=production EXTRA=two bash -c echo second", rec2.Result())
	})

	t.Run("derived command does not mutate template environ", func(t *testing.T) {
		runtime := shell.Runtime().As("egd").Environ("BASE", "value")

		// Create a derived command that adds an env var
		_ = runtime.New("echo derived").Environ("ADDED", "extra")

		// Create another derived command from the original template
		cmd := runtime.New("echo original")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(ctx, cmd))
		require.Equal(t, ":BASE=value:sudo:-H -u egd -g egd env BASE=value bash -c echo original", rec.Result())
	})

	t.Run("multiple commands record sequentially", func(t *testing.T) {
		runtime := shell.Runtime().As("postgres").Environ("PAGER", "")
		cmd1 := runtime.New("pg_isready")
		cmd2 := runtime.New("psql -c \"SELECT 1\"")

		rec1 := shell.NewRecorder(&cmd1)
		rec2 := shell.NewRecorder(&cmd2)
		require.NoError(t, shell.Run(ctx, cmd1, cmd2))
		require.Equal(t, ":PAGER=:sudo:-H -u postgres -g postgres env PAGER= bash -c pg_isready", rec1.Result())
		require.Equal(t, ":PAGER=:sudo:-H -u postgres -g postgres env PAGER= bash -c psql -c \"SELECT 1\"", rec2.Result())
	})
}
