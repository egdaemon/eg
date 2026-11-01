package maintainer

import "github.com/egdaemon/eg/runtime/wasi/eg"

const (
	Name           = "engineering"
	Email          = "engineering@egdaemon.com"
	GPGFingerprint = "394332CE06C035E40237A6743A33D4F9C8E3829F"
)

func Container() eg.C8S {
	return eg.Container("eg.ubuntu").BuildFromFile(".eg/Containerfile")
}
