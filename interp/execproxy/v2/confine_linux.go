package execproxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/egdaemon/eg"
	"golang.org/x/sys/unix"
)

// stageconfine is the name the proxy starts itself under to run a command it confines: the stage
// restricts itself with Landlock, then execs the command, which keeps the restriction.
const stageconfine = "egexec-confine"

// pathbeneath is LANDLOCK_RULE_PATH_BENEATH, the rule granting access beneath a directory.
const pathbeneath = 1

func init() {
	var (
		err error
	)

	if os.Args[0] != stageconfine {
		return
	}

	if err = confine(eg.DefaultRootDirectory(), eg.DefaultMountRoot(), "/tmp", "/var/tmp", "/dev"); err == nil {
		err = unix.Exec(os.Args[1], os.Args[2:], os.Environ())
	}
	fmt.Fprintln(os.Stderr, "egexec:", err)
	os.Exit(127)
}

// confined is the process that runs name with args: the command itself when sudo runs it as root,
// which a guest asks for with Privileged() to change the system, otherwise a stage confining it to
// writing eg's directories (the workload's, and the mounts they are bound from), the temporary
// directories and the devices. It reads and runs anything else.
// A name the module's PATH does not hold runs as it is, and fails to start.
func confined(ctx context.Context, name string, args ...string) *exec.Cmd {
	path, err := exec.LookPath(name)
	if err != nil || privileged(name, args) {
		return exec.CommandContext(ctx, name, args...)
	}

	cmd := exec.CommandContext(ctx, "/proc/self/exe", append([]string{path, name}, args...)...)
	cmd.Args[0] = stageconfine
	return cmd
}

// privileged reports whether name with args is sudo running a command as root.
func privileged(name string, args []string) bool {
	if filepath.Base(name) != "sudo" {
		return false
	}

	for i, arg := range args {
		if user, ok := strings.CutPrefix(arg, "--user="); ok {
			return user == "root"
		}
		if (arg == "-u" || arg == "--user") && i+1 < len(args) {
			return args[i+1] == "root"
		}
	}

	return false
}

// confine restricts the process with Landlock to changing the filesystem beneath dirs, as far as the
// kernel's Landlock does; a kernel without it leaves the process as it is. Restricting itself needs
// CAP_SYS_ADMIN, which a module running as root holds, or no new privileges, which keeps sudo from
// switching users, so only a process without the capability takes no new privileges.
func confine(dirs ...string) (err error) {
	var (
		abi     uintptr
		ruleset uintptr
		errno   unix.Errno
		fd      int
		attr    unix.LandlockRulesetAttr
	)

	if abi, _, errno = unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION); errno == unix.ENOSYS || errno == unix.EOPNOTSUPP {
		return nil
	} else if errno != 0 {
		return fmt.Errorf("unable to query landlock: %w", errno)
	}

	attr.Handled_access_fs = writes(abi)
	if ruleset, _, errno = unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0); errno != 0 {
		return fmt.Errorf("unable to create the landlock ruleset: %w", errno)
	}
	defer unix.Close(int(ruleset))

	for _, dir := range dirs {
		if fd, err = unix.Open(dir, unix.O_PATH|unix.O_CLOEXEC, 0); errors.Is(err, unix.ENOENT) {
			continue
		} else if err != nil {
			return fmt.Errorf("unable to open %s: %w", dir, err)
		}

		beneath := unix.LandlockPathBeneathAttr{Allowed_access: attr.Handled_access_fs, Parent_fd: int32(fd)}
		_, _, errno = unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, ruleset, pathbeneath, uintptr(unsafe.Pointer(&beneath)), 0, 0, 0)
		unix.Close(fd)
		if errno != 0 {
			return fmt.Errorf("unable to permit changes beneath %s: %w", dir, errno)
		}
	}

	if _, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, ruleset, 0, 0); errno == unix.EPERM {
		if err = unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("unable to take no new privileges: %w", err)
		}
		_, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, ruleset, 0, 0)
	}
	if errno != 0 {
		return fmt.Errorf("unable to restrict the command: %w", errno)
	}

	return nil
}

// writes are the rights that change the filesystem in Landlock abi version abi, which a ruleset
// handles so that only its rules grant them.
func writes(abi uintptr) uint64 {
	var (
		rights uint64 = unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
			unix.LANDLOCK_ACCESS_FS_MAKE_CHAR | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_MAKE_REG |
			unix.LANDLOCK_ACCESS_FS_MAKE_SOCK | unix.LANDLOCK_ACCESS_FS_MAKE_FIFO | unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_SYM
	)

	if abi >= 2 {
		rights |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		rights |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}

	return rights
}
