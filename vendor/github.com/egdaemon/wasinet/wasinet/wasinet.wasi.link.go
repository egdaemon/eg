//go:build wasip1

package wasinet

import (
	_ "unsafe"
)

// //go:linkname syscall_derp syscall.fd_fdstat_get_type
// func syscall_derp(fd int) (uint8, error)

// //go:linkname derp net.fd_fdstat_get_type
//
//	func derp(fd int) (uint8, error) {
//		panic("okay")
//		fmt.Println("DERP DERP", fd)
//		return 0, nil
//	}

// //go:linkname timeNow time.Now
// func timeNow() time.Time {
// 	return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
// }
