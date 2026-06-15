//go:build linux

package runners

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/jaypipes/ghw"
	"github.com/logrusorgru/aurora"
)

func AgentOptionHostOS(cli ...string) AgentOption {
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", "host"),       // properly map host user into containers.
		AgentOptionCommandLine("--cap-add", "NET_ADMIN"), // required for loopback device creation inside the container
		AgentOptionCommandLine("--cap-add", "SYS_ADMIN"), // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		AgentOptionCommandLine("--device", "/dev/fuse"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		AgentOptionCommandLine("--pids-limit", "-1"),     // ensure we dont run into pid limits.
		AgentOptionCommandLine(cli...),                   // escape hatch to allow customizing the container cli
	)
}

// DetectGPU returns the driver and vram of the gpu with the most vram available on the host, if any.
// the zero values are returned when no gpu (or its vram) could be detected.
func DetectGPU() (driver string, vram uint64, err error) {
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

		if cvram > vram {
			driver, vram = card.DeviceInfo.Driver, cvram
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
