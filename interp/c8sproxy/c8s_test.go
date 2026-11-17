package c8sproxy

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanup(t *testing.T) {
	t.Run("stops gracefully before forcibly removing the container", func(t *testing.T) {
		var invoked [][]string

		cleanup(context.Background(), func(c *exec.Cmd) *exec.Cmd {
			invoked = append(invoked, c.Args)
			return exec.Command("true")
		}, "eg-example")

		assert.Equal(t, [][]string{
			{"podman", "stop", "eg-example"},
			{"podman", "rm", "--force", "--ignore", "eg-example"},
		}, invoked)
	})

	t.Run("removes the container even when stop fails", func(t *testing.T) {
		var invoked [][]string

		cleanup(context.Background(), func(c *exec.Cmd) *exec.Cmd {
			invoked = append(invoked, c.Args)
			// every command fails, the container must still be removed or its published ports leak.
			return exec.Command("false")
		}, "eg-example")

		assert.Equal(t, [][]string{
			{"podman", "stop", "eg-example"},
			{"podman", "rm", "--force", "--ignore", "eg-example"},
		}, invoked)
	})

	t.Run("removes the container even when stop exceeds its budget", func(t *testing.T) {
		var invoked [][]string

		// a canceled parent context expires both budgets immediately, equivalent to stop running out of time.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		cleanup(ctx, func(c *exec.Cmd) *exec.Cmd {
			invoked = append(invoked, c.Args)
			return exec.CommandContext(ctx, "true")
		}, "eg-example")

		assert.Len(t, invoked, 2)
		assert.Equal(t, []string{"podman", "rm", "--force", "--ignore", "eg-example"}, invoked[1])
	})

	t.Run("a container publishing ports that does not stop gracefully is force removed", func(t *testing.T) {
		var invoked [][]string

		// the options runners.AgentOptionPublish(3000) gives the module container for eg compute serve. written
		// out since runners imports this package.
		run := PodmanModuleRunCmd("image", "eg-example", "--publish", "3000:3000")
		assert.Contains(t, run, "--publish")
		assert.Contains(t, run, "3000:3000")

		cleanup(context.Background(), func(c *exec.Cmd) *exec.Cmd {
			invoked = append(invoked, c.Args)
			if c.Args[1] == "stop" {
				// the workload ignores SIGTERM, so stop exits nonzero once its grace period ends and
				// the container, along with the pasta process holding port 3000, is left behind.
				return exec.Command("sh", "-c", "exit 125")
			}
			return exec.Command("true")
		}, run[2])

		assert.Equal(t, [][]string{
			{"podman", "stop", "eg-example"},
			{"podman", "rm", "--force", "--ignore", "eg-example"},
		}, invoked)
	})

	// runs a real container, needs podman. the container ignores SIGTERM and publishes a port, as a workload
	// of eg compute serve --ports does; after cleanup the container must be gone and the port released.
	t.Run("a published port is released once a container that ignores SIGTERM is cleaned up", func(t *testing.T) {
		if testing.Short() {
			t.Skip("stopping a container that ignores SIGTERM takes its full grace period")
		}
		t.SkipNow() // some thing is busted in the network layer.

		if _, err := exec.LookPath("podman"); err != nil {
			t.Skip("podman is not available")
		}

		// publishing a port makes rootless podman start pasta, which needs a tap device.
		if _, err := os.Stat("/dev/net/tun"); err != nil {
			t.Skip("/dev/net/tun is not available, required by pasta to publish ports")
		}

		// an unused port, so a leaked container from another run or a real workload doesn't interfere.
		l, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		port := l.Addr().(*net.TCPAddr).Port
		require.NoError(t, l.Close())

		cname := fmt.Sprintf("eg-cleanup-test-%d", port)
		image := fmt.Sprintf("localhost/%s", cname)

		out, err := exec.Command("podman", "build", "--tag", image, ".xfixtures/sigtermignore").CombinedOutput()
		require.NoError(t, err, string(out))
		t.Cleanup(func() { exec.Command("podman", "rmi", "--force", image).Run() })
		// belt and braces so a failing assertion never leaks the container this test created.
		t.Cleanup(func() { exec.Command("podman", "rm", "--force", "--ignore", "--time", "0", cname).Run() })

		out, err = exec.Command("podman", PodmanModuleRunCmd(image, cname, "--publish", fmt.Sprintf("%d:%d", port, port))...).CombinedOutput()
		require.NoError(t, err, string(out))

		// the published port is held while the container runs.
		_, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		require.Error(t, err, "expected the port to be held by the running container")

		cleanup(context.Background(), func(c *exec.Cmd) *exec.Cmd { return c }, cname)

		out, err = exec.Command("podman", "container", "exists", cname).CombinedOutput()
		require.Error(t, err, "container still exists: %s", string(out))

		// pasta exits shortly after the container is removed, so allow it a moment to release the port.
		require.Eventually(t, func() bool {
			l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return false
			}
			return l.Close() == nil
		}, 5*time.Second, 100*time.Millisecond, "port was not released")
	})
}
