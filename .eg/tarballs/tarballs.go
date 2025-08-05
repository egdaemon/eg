package tarballs

import (
	"fmt"

	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

func Eg(os, arch string) string {
	return egtarball.GitPattern(fmt.Sprintf("eg.%s.%s", os, arch))
}

func EgDarwinArm64() string {
	return Eg("darwin", "arm64")
}
