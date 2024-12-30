//go:build wasip1

package wasinet

// func netaddrfamily(addr net.Addr) int {
// 	translated := func(v int32) int {
// 		return int(sock_determine_host_af_family(v))
// 	}
// 	ipfamily := func(ip net.IP) int {
// 		if ip.To4() == nil {
// 			return translated(syscall.AF_INET6)
// 		}

// 		return translated(syscall.AF_INET)
// 	}

// 	switch a := addr.(type) {
// 	case *net.IPAddr:
// 		return ipfamily(a.IP)
// 	case *net.TCPAddr:
// 		return ipfamily(a.IP)
// 	case *net.UDPAddr:
// 		return ipfamily(a.IP)
// 	case *net.UnixAddr:
// 		return translated(syscall.AF_UNIX)
// 	}

// 	return translated(syscall.AF_INET)
// }

// var (
// 	maponce     sync.Once
// 	hostAFINET6 = int32(syscall.AF_INET6)
// 	hostAFINET  = int32(syscall.AF_INET)
// 	hostAFUNIX  = int32(syscall.AF_UNIX)
// )

// func rawtosockaddr(rsa *rawsocketaddr) (sockaddr, error) {
// 	maponce.Do(func() {
// 		hostAFINET = sock_determine_host_af_family(hostAFINET)
// 		hostAFINET6 = sock_determine_host_af_family(hostAFINET6)
// 		hostAFUNIX = sock_determine_host_af_family(hostAFUNIX)
// 	})

// 	switch int32(rsa.family) {
// 	case hostAFINET:
// 		addr := (*sockipaddr[sockip4])(unsafe.Pointer(&rsa.addr))
// 		return addr, nil
// 	case hostAFINET6:
// 		addr := (*sockipaddr[sockip6])(unsafe.Pointer(&rsa.addr))
// 		return addr, nil
// 	case hostAFUNIX:
// 		addr := (*sockaddrUnix)(unsafe.Pointer(&rsa.addr))
// 		return addr, nil
// 	default:
// 		log.Println("unable to determine socket family", rsa.family)
// 		return nil, syscall.ENOTSUP
// 	}
// }
