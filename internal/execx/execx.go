package execx

import (
	"bytes"
	"context"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

func MaybeRun(c *exec.Cmd) error {
	if c == nil {
		return nil
	}

	debugx.Println("---------------", errorsx.Must(os.Getwd()), envx.String("", eg.EnvComputeRunID), "running", c.Dir, "->", c.String(), "---------------")
	return c.Run()
}

// ErrNotFound is the error resulting if a path search failed to find an executable file.
const ErrNotFound = errorsx.String("executable file not found in $path")

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		debugx.Println("finding failed", err)
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	log.Println("finding failed permission")
	return fs.ErrPermission
}

// LookPath implementation from golang stdlib due to their
// noop implementation for wasm.
func LookPath(file string) (string, error) {
	// skip the path lookup for these prefixes
	skip := []string{"/", "#", "./", "../"}

	for _, p := range skip {
		if strings.HasPrefix(file, p) {
			err := findExecutable(file)
			if err == nil {
				return file, nil
			}
			return "", &exec.Error{Name: file, Err: err}
		}
	}

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			if !filepath.IsAbs(path) {
				return path, &exec.Error{Name: file, Err: exec.ErrDot}
			}
			return path, nil
		}
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

// RunAs builds an *exec.Cmd for name/args that runs as the named OS
// user via sudo -- used when a target path is owned by a different uid than
// this process (e.g. a repository bind-mounted into a workload container
// under a service account), where running directly would trip permission or
// ownership checks. Falls back to running directly (no sudo) when username
// doesn't resolve to a system user, or already matches the current uid.
func RunAs(ctx context.Context, username string, name string, args ...string) *exec.Cmd {
	u, err := user.Lookup(username)
	if err != nil {
		return exec.CommandContext(ctx, name, args...)
	}

	if uid, err := strconv.Atoi(u.Uid); err == nil && uid == os.Getuid() {
		return exec.CommandContext(ctx, name, args...)
	}

	return exec.CommandContext(ctx, "sudo", append([]string{"-u", u.Username, "-g", u.Username, "--", name}, args...)...)
}

func String(ctx context.Context, prog string, args ...string) (_ string, err error) {
	var (
		buf bytes.Buffer
	)

	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Stdout = &buf

	if err = cmd.Run(); err != nil {
		return "", err
	}

	return buf.String(), nil
}
