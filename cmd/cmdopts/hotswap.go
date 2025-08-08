package cmdopts

import (
	"log"
	"os"
	"runtime"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/langx"
)

// injectable
type HotswapPath string

func (t *HotswapPath) String() string {
	return langx.Autoderef((*string)(t))
}

// flag used to locate and configure the environment to swap out the
// binary used in containers with the local version.
type Hotswap struct {
	Hotswap bool `name:"hotswap" help:"replace the eg binary in containers with the version used to run the command" hidden:"true" default:"false"`
}

func (t Hotswap) AfterApply(dst *HotswapPath) error {
	if !t.Hotswap {
		return nil
	}

	if runtime.GOOS == "darwin" {
		log.Println("hotswapping the binary is not supported on darwin, results in permission denied errors when executing the binary.")
		return nil
	}

	path := eg.DefaultMountRoot(eg.RuntimeDirectory, eg.BinaryBin)
	os.Setenv(eg.EnvComputeBin, path)
	*dst = HotswapPath(path)

	return nil
}
