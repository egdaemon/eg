package c8sproxy

import (
	"context"
	"log"
	"testing"

	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/machines"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

type machineserver struct {
	machines.UnimplementedMachinesServer
	requests []*machines.ModuleRequest
}

func (t *machineserver) Module(ctx context.Context, req *machines.ModuleRequest) (*machines.ModuleResponse, error) {
	t.requests = append(t.requests, req)
	return &machines.ModuleResponse{}, nil
}

func TestEnviron(t *testing.T) {
	t.Run("keeps a variable with its value", func(t *testing.T) {
		require.Equal(t, []string{"A=1", "B=2"}, environ([]string{"-e", "A=1", "--env", "B=2"}))
	})

	t.Run("gives a variable without a value the one it has here", func(t *testing.T) {
		t.Setenv("EG_TEST_ENVIRON", "here")
		require.Equal(t, []string{"EG_TEST_ENVIRON=here"}, environ([]string{"-e", "EG_TEST_ENVIRON"}))
	})

	t.Run("drops a variable without a value that is unset here", func(t *testing.T) {
		require.Empty(t, environ([]string{"-e", "EG_TEST_ENVIRON_UNSET"}))
	})

	t.Run("ignores options that are not environment", func(t *testing.T) {
		require.Equal(t, []string{"A=1"}, environ([]string{"--volume", "/a:/b", "-e", "A=1", "-w", "/c"}))
	})

	t.Run("ignores a trailing flag without its value", func(t *testing.T) {
		require.Empty(t, environ([]string{"-e"}))
	})
}

func TestServiceProxyMachines(t *testing.T) {
	proxy := func(t *testing.T, m machines.MachinesServer) *ProxyService {
		return NewServiceProxy(log.Default(), workspaces.Context{Root: t.TempDir(), WorkingDir: t.TempDir()}, ServiceProxyOptionMachines(m))
	}

	t.Run("the root container is not built", func(t *testing.T) {
		_, err := proxy(t, &machineserver{}).Build(t.Context(), &c8s.BuildRequest{Name: "eg", Definition: "missing/Containerfile"})
		require.NoError(t, err)
	})

	t.Run("the root container's modules run in machines with their environment", func(t *testing.T) {
		m := &machineserver{}

		_, err := proxy(t, m).Module(t.Context(), &c8s.ModuleRequest{Image: "eg", Name: "eg.x", Module: "module.wasm", Options: []string{"-e", "EG_COMPUTE_MODULE_NESTED_LEVEL=1"}})
		require.NoError(t, err)
		require.Len(t, m.requests, 1)
		require.Equal(t, "module.wasm", m.requests[0].Module)
		require.Equal(t, []string{"EG_COMPUTE_MODULE_NESTED_LEVEL=1"}, m.requests[0].Options)
		require.Empty(t, m.requests[0].Egiso)
	})

	t.Run("other containers are built with podman", func(t *testing.T) {
		_, err := proxy(t, &machineserver{}).Build(t.Context(), &c8s.BuildRequest{Name: "other", Definition: "missing/Containerfile"})
		require.Error(t, err)
	})
}
