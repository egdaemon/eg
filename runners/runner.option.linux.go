//go:build linux

package runners

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/jaypipes/ghw"
	"github.com/logrusorgru/aurora"
)

func AgentOptionHostOS(cli ...string) AgentOption {
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", envx.String("host", eg.EnvComputePodmanUsernamespace)),                                                // properly map host user into containers.
		envx.Toggle(AgentOptionNoop, AgentOptionCommandLine("--cap-add", "NET_ADMIN"), envx.Boolean(true, eg.EnvComputePodmanCapNetwork)),        // required for loopback device creation inside the container
		envx.Toggle(AgentOptionNoop, AgentOptionCommandLine("--cap-add", "SYS_ADMIN"), envx.Boolean(true, eg.EnvComputePodmanCapAdmin)),          // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		envx.Toggle(AgentOptionNoop, AgentOptionCommandLine("--device", "/dev/fuse"), envx.Boolean(true, eg.EnvComputePodmanDeviceFuse)),         // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		envx.Toggle(AgentOptionNoop, AgentOptionCommandLine("--device", "/dev/net/tun"), envx.Boolean(true, eg.EnvComputePodmanDeviceTun)),       // required for pasta (rootless container networking) to create a tap device when containers run inside the workload.
		AgentOptionCommandLine("--security-opt", envx.String("label=disable", eg.EnvComputePodmanSecurityLabel)),                                 // as eg-runner.service, container_init_t refuses the mounts systemd's units sandbox themselves with.
		AgentOptionCommandLine("--pids-limit", envx.String("-1", eg.EnvComputePodmanPidsLimit)),                                                  // ensure we dont run into pid limits.
		envx.Toggle(AgentOptionNoop, AgentOptionCommandLine("--security-opt", "unmask=/proc/*"), envx.Boolean(false, eg.EnvComputeRunnerUnmask)), // opt in: nested containers can't mount a fresh /proc over podman's masked one.
		AgentOptionCommandLine(cli...), // escape hatch to allow customizing the container cli
	)
}

// DetectGPU returns the driver and vram of the gpu with the most vram available on the host, if any.
// the zero values are returned when no gpu (or its vram) could be detected.
func DetectGPU() (driver string, vram bytesx.Unit, err error) {
	gpus, err := ghw.GPU()
	if err != nil {
		return "", 0, errorsx.Wrap(err, "unable to determine hardware capability")
	}

	for _, card := range gpus.GraphicsCards {
		if card.DeviceInfo == nil {
			continue
		}

		raw, err := os.ReadFile(filepath.Join("/sys/bus/pci/devices", card.DeviceInfo.Address, "mem_info_vram_total"))
		if err != nil {
			continue
		}

		cvram, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
		if err != nil {
			continue
		}

		if _cvram := bytesx.Unit(cvram); _cvram > vram {
			driver, vram = card.DeviceInfo.Driver, _cvram
		}
	}

	return driver, vram, nil
}

func AgentOptionGPU(enabled bool) (AgentOption, error) {
	if !enabled {
		return AgentOptionNoop, nil
	}

	log.Println(aurora.NewAurora(true).Yellow("info: gpu support is currently experimental"))

	driver, _, err := DetectGPU()
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to determine hardware capability")
	}

	device := GPUDeviceSpec(driver)
	if device == "" && driver != "" {
		log.Println("unknown driver, file an issue", driver)
	}

	return AgentOptionCompose(
		AgentOptionVolumes(
			AgentMountReadOnly("/etc/cdi", "/etc/cdi"),
		),
		AgentOptionCommandLine("--device", device),
	), nil
}
