// Package proc is where a process's stdio sockets live within a runtime directory: the guest listens on them in
// proc/{id}, and whatever runs the process connects to them.
package proc

import (
	"net"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
)

// Names are the sockets of a process within proc/{id}, in the order they are connected to.
var Names = []string{"stdin", "stdout", "stderr"}

// Dir is the directory of the process id's sockets within runtimedir.
func Dir(runtimedir string, id string) string {
	return filepath.Join(runtimedir, "proc", id)
}

// Dial connects to the guest's stdin, stdout and stderr sockets for the process id within runtimedir. the
// sockets are handed to the process as files, so its output reaches the guest without passing through the caller.
func Dial(runtimedir string, id string) (files []*os.File, err error) {
	var (
		conn net.Conn
		f    *os.File
	)

	if !filepath.IsLocal(id) || filepath.Base(id) != id {
		return nil, errorsx.Errorf("invalid process id: %q", id)
	}

	for _, name := range Names {
		if conn, err = net.Dial("unix", filepath.Join(Dir(runtimedir, id), name)); err != nil {
			break
		}

		f, err = conn.(*net.UnixConn).File()
		conn.Close()
		if err != nil {
			break
		}

		files = append(files, f)
	}

	if err != nil {
		for _, f = range files {
			f.Close()
		}
		return nil, errorsx.Wrapf(err, "unable to connect the stdio of process %s", id)
	}

	return files, nil
}
