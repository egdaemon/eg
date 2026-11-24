package main

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestModuleArgs(t *testing.T) {
	// the command line of eg has global flags, e.g.) -v, which a module's arguments must not be mistaken for.
	type cli struct {
		Verbosity int    `short:"v" type:"counter"`
		Module    module `cmd:""`
	}

	parse := func(t *testing.T, args ...string) module {
		var c cli
		parser, err := kong.New(&c, kong.Vars{
			"vars_eg_root_directory":        "/w",
			"vars_workload_directory":       ".eg",
			"vars_eg_runtime_directory":     "/r",
			"env_eg_compute_module_isolate": eg.EnvComputeModuleIsolate,
		})
		require.NoError(t, err)
		_, err = parser.Parse(append([]string{"module"}, args...))
		require.NoError(t, err)
		return c.Module
	}

	t.Run("without arguments", func(t *testing.T) {
		m := parse(t, "/m.wasm")
		require.Equal(t, "/m.wasm", m.Module)
		require.Empty(t, m.Args)
	})

	t.Run("arguments after -- are the module's, flags included", func(t *testing.T) {
		m := parse(t, "--directory", "/d", "/m.wasm", "--", "compute", "local", "-vv", "--secret", "a b", "--directory=/x")
		require.Equal(t, "/d", m.Dir)
		require.Equal(t, "/m.wasm", m.Module)
		require.Equal(t, []string{"compute", "local", "-vv", "--secret", "a b", "--directory=/x"}, m.Args)
	})

	t.Run("the arguments may start with a flag", func(t *testing.T) {
		m := parse(t, "/m.wasm", "--", "--secret", "a", "-vv", "release/egcoreutils")
		require.Equal(t, []string{"--secret", "a", "-vv", "release/egcoreutils"}, m.Args)
	})

	t.Run("the arguments may start with a flag the command knows", func(t *testing.T) {
		m := parse(t, "/m.wasm", "--", "-vv", "release/egcoreutils")
		require.Equal(t, []string{"-vv", "release/egcoreutils"}, m.Args)
	})
}
