package egiso

import (
	"os"
	"sync"
)

// hostdir is a host directory whose Lstat resolves a name with one call where the platform can, as
// os.Root opens every directory on the way to it. It opens the directory itself on its first Lstat,
// and the descriptor closes with the last copy of the hostdir holding it.
type hostdir struct {
	*os.Root
	self func() (*os.File, error)
}

func newhostdir(root *os.Root) hostdir {
	return hostdir{Root: root, self: sync.OnceValues(func() (*os.File, error) { return root.Open(".") })}
}
