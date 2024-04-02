package systemx

import (
	"log"
	"net"
	"os"
)

// HostnameOrLocalhost returns the hostname, otherwise fallsback to localhost.
func HostnameOrLocalhost() string {
	const localhost = "localhost"
	return HostnameOrDefault(localhost)
}

// HostnameOrDefault returns the hostname, or the provided fallback.
func HostnameOrDefault(fallback string) string {
	var (
		err      error
		hostname string
	)

	if hostname, err = os.Hostname(); err != nil {
		log.Println("failed to get hostname", err)
		return fallback
	}

	return hostname
}

// WorkingDirectoryOrDefault loads the working directory or fallsback to the provided
// path when an error occurs.
func WorkingDirectoryOrDefault(fallback string) (dir string) {
	var (
		err error
	)

	if dir, err = os.Getwd(); err != nil {
		log.Println("failed to get working directory", err)
		return fallback
	}

	return dir
}

// HostIP ...
func HostIP(host string) net.IP {
	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		log.Println("failed to resolve ip for", host, "falling back to 127.0.0.1:", err)
		return net.ParseIP("127.0.0.1")
	}

	return ip.IP
}
