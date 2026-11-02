package c8sproxy

import (
	"log"
	"testing"

	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/assert"
)

func TestServiceProxyBuildOptions(t *testing.T) {
	req := &c8s.BuildRequest{Options: []string{"--build-arg", "A=1"}}

	t.Run("without build options only those of the request are used", func(t *testing.T) {
		proxy := NewServiceProxy(log.Default(), workspaces.Context{})
		assert.Equal(t, []string{"--build-arg", "A=1"}, proxy.buildoptions(req))
	})

	t.Run("build options are applied before those of the request", func(t *testing.T) {
		proxy := NewServiceProxy(log.Default(), workspaces.Context{}, ServiceProxyOptionBuildOptions("--network", "host"))
		assert.Equal(t, []string{"--network", "host", "--build-arg", "A=1"}, proxy.buildoptions(req))
	})

	t.Run("container options are not applied to builds", func(t *testing.T) {
		proxy := NewServiceProxy(log.Default(), workspaces.Context{}, ServiceProxyOptionContainerOptions("--volume", "/a:/b", "--cgroups", "disabled"))
		assert.Equal(t, []string{"--build-arg", "A=1"}, proxy.buildoptions(req))
	})

	t.Run("the options of the request are not modified", func(t *testing.T) {
		proxy := NewServiceProxy(log.Default(), workspaces.Context{}, ServiceProxyOptionBuildOptions("--network", "host"))
		proxy.buildoptions(req)
		assert.Equal(t, []string{"--build-arg", "A=1"}, req.Options)
	})
}
