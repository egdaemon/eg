//go:build linux

package runners

import (
	"log"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/gpu"
	"github.com/jaypipes/ghw/pkg/pci"
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

func AgentOptionGPU(enabled bool) (AgentOption, error) {
	if !enabled {
		return AgentOptionNoop, nil
	}

	log.Println(aurora.NewAurora(true).Yellow("info: gpu support is currently experimental"))

	gpus, err := ghw.GPU()
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to determine hardware capability")
	}

	driver := slicesx.Reduce(
		func(device string, info *pci.Device) string {
			if stringsx.Present(device) {
				return device
			}

			switch info.Driver {
			case "amdgpu":
				return "amd.com/gpu=all"
			default:
				log.Println("unknown driver, file an issue", spew.Sdump(info))
				return device
			}
		},
		"",
		slicesx.MapTransform(func(v *gpu.GraphicsCard) *pci.Device {
			return v.DeviceInfo
		}, gpus.GraphicsCards...)...,
	)

	return AgentOptionCompose(
		AgentOptionVolumes(
			AgentMountReadOnly("/etc/cdi", "/etc/cdi"),
		),
		AgentOptionCommandLine("--device", driver),
	), nil
}
