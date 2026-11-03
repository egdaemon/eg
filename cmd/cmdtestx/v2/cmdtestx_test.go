package cmdtestx_test

import (
	"os"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	cmdtestx "github.com/egdaemon/eg/cmd/cmdtestx/v2"
	"github.com/stretchr/testify/require"
)

type echo struct {
	Name string `arg:""`
	out  *string
}

func (t echo) Run(ctx *cmdopts.Global) error {
	if ctx.Context == nil || ctx.Cleanup == nil || ctx.Shutdown == nil {
		panic("global not bound")
	}
	*t.out = t.Name
	return nil
}

type fromenv struct {
	Name string `name:"name" env:"${env_name}"`
	out  *string
}

func (t fromenv) Run() error {
	*t.out = t.Name
	return nil
}

func TestRun(t *testing.T) {
	t.Run("missing argument fails to parse", func(t *testing.T) {
		var out string
		require.Error(t, cmdtestx.New(t, &echo{out: &out}).Run())
	})

	t.Run("runs the command with global bound", func(t *testing.T) {
		var out string
		require.NoError(t, cmdtestx.New(t, &echo{out: &out}).Run("foo"))
		require.Equal(t, "foo", out)
	})
}

func TestEnv(t *testing.T) {
	vars := kong.Vars{"env_name": "CMDTESTX_NAME"}

	t.Run("provided without the process environment", func(t *testing.T) {
		var out string
		require.NoError(t, cmdtestx.New(t, &fromenv{out: &out}).Options(vars).Env("CMDTESTX_NAME=bar").Run())
		require.Equal(t, "bar", out)
		_, ok := os.LookupEnv("CMDTESTX_NAME")
		require.False(t, ok)
	})

	t.Run("last value wins", func(t *testing.T) {
		var out string
		require.NoError(t, cmdtestx.New(t, &fromenv{out: &out}).Options(vars).Env("CMDTESTX_NAME=a").Env("CMDTESTX_NAME=b").Run())
		require.Equal(t, "b", out)
	})

	t.Run("command line wins", func(t *testing.T) {
		var out string
		require.NoError(t, cmdtestx.New(t, &fromenv{out: &out}).Options(vars).Env("CMDTESTX_NAME=bar").Run("--name=baz"))
		require.Equal(t, "baz", out)
	})

	t.Run("malformed entry fails", func(t *testing.T) {
		var out string
		require.Error(t, cmdtestx.New(t, &fromenv{out: &out}).Options(vars).Env("CMDTESTX_NAME").Run())
	})
}
