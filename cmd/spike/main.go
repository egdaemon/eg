package main

import (
	"github.com/egdaemon/eg/internal/gitx"
)

func main() {
	gitx.Remote("origin", ".")
}
