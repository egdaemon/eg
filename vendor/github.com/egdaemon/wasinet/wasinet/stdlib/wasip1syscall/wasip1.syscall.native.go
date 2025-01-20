//go:build !wasip1

package wasip1syscall

var (
	afmap = _AFFamilyMap{}
)

func AF() _AFFamilyMap {
	return afmap
}
