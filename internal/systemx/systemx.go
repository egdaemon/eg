package systemx

import (
	"crypto/md5"
	"encoding/hex"
	"log"
	"net"
	"os"
	"os/user"
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

// MustUser ...
func MustUser() *user.User {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}

	return u
}

// CurrentUserOrDefault returns the current user or the default configured user.
// (usually root)
func CurrentUserOrDefault(d user.User) (result *user.User) {
	var (
		err error
	)

	if result, err = user.Current(); err != nil {
		log.Println("failed to retrieve current user, using default", err)
		tmp := d
		return &tmp
	}

	return result
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

// FileExists returns true IFF a non-directory file exists at the provided path.
func FileExists(path string) bool {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	if info.IsDir() {
		return false
	}

	return true
}

// FileMD5 computes digest of file contents.
// if something goes wrong logs and returns an empty string.
func FileMD5(path string) string {
	var (
		err  error
		read []byte
	)

	if read, err = os.ReadFile(path); err != nil {
		log.Println("digest failed", err)
		return ""
	}

	digest := md5.Sum(read)

	return hex.EncodeToString(digest[:])
}
