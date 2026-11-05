package shell

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/egdaemon/sh/v3/syntax"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// recorder is a module that records the requests it receives and runs nothing.
type recorder struct {
	requests []*v1.ExecRequest
}

func (t *recorder) Exec(_ context.Context, req *v1.ExecRequest, _ ...grpc.CallOption) (*v1.ExecResponse, error) {
	t.requests = append(t.requests, req)
	return &v1.ExecResponse{}, nil
}

func TestCoreutils(t *testing.T) {
	// interpret runs cmd with the handlers as user egd, and returns the requests that reached the module.
	interpret := func(t *testing.T, cmd string) []*v1.ExecRequest {
		// unix socket paths are length limited, t.TempDir can exceed it.
		runtimedir, err := os.MkdirTemp("", "shell")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(runtimedir) })
		t.Setenv(eg.EnvComputeRuntimeDirectory, runtimedir)
		t.Setenv(eg.EnvComputeWorkingDirectory, runtimedir)

		file, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
		require.NoError(t, err)

		c := &recorder{}
		r, err := interp.New(interp.Dir(runtimedir), interp.ExecHandlers(handlers(c, "egd", "egd")...))
		require.NoError(t, err)
		require.NoError(t, r.Run(t.Context(), file))
		return c.requests
	}

	t.Run("runs utilities in coreutils.wasm as the module's user", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalCoreutils, "true")
		reqs := interpret(t, "cat file | sort -r")
		require.Len(t, reqs, 2)
		for _, req := range reqs {
			require.Equal(t, "coreutils", req.Wasm)
		}
		require.ElementsMatch(t, []string{"cat", "sort"}, []string{reqs[0].Cmd, reqs[1].Cmd})
	})

	t.Run("passes the utility's arguments", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalCoreutils, "true")
		reqs := interpret(t, "mkdir -p a/b")
		require.Len(t, reqs, 1)
		require.Equal(t, "mkdir", reqs[0].Cmd)
		require.Equal(t, []string{"-p", "a/b"}, reqs[0].Arguments)
	})

	t.Run("runs other commands natively as the user", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalCoreutils, "true")
		reqs := interpret(t, "go version")
		require.Len(t, reqs, 1)
		require.Empty(t, reqs[0].Wasm)
		require.Equal(t, "sudo", reqs[0].Cmd)
		require.Equal(t, []string{"-E", "-H", "-u", "egd", "-g", "egd", "go", "version"}, reqs[0].Arguments)
	})

	t.Run("runs nice and realpath natively", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalCoreutils, "true")
		for _, req := range interpret(t, "nice true; realpath file") {
			require.Empty(t, req.Wasm)
		}
	})

	t.Run("runs utilities natively without the experiment", func(t *testing.T) {
		t.Setenv(eg.EnvExperimentalCoreutils, "")
		reqs := interpret(t, "cat file")
		require.Len(t, reqs, 1)
		require.Empty(t, reqs[0].Wasm)
		require.Equal(t, "sudo", reqs[0].Cmd)
	})
}
