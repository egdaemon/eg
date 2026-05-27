//go:build darwin && arm64

package macvmproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/interp/macvm"
	"golang.org/x/crypto/ssh"
)

const (
	shareWorking     = "working"
	shareCache       = "cache"
	shareRuntime     = "runtime"
	shareWorkspace   = "workspace"
	shareEGBin       = "egbin"
	guestSharesMount = "/Volumes/My Shared Files"
	guestSSHUser     = "admin"
	guestSSHPass     = "admin"
)

var (
	vmsmu sync.Mutex
	vms   = map[string]*exec.Cmd{}
)

// Pull clones the named source image into a local VM named req.Name. Tart's
// `clone` implicitly pulls when the source is a remote `:tag`; the existing-VM
// short-circuit in tartClone makes the call idempotent across re-runs.
func (t *ProxyService) Pull(ctx context.Context, req *macvm.PullRequest) (*macvm.PullResponse, error) {
	debugx.Println("PROXY MACVM PULL INITIATED", req.Name, req.Image)
	defer debugx.Println("PROXY MACVM PULL COMPLETED", req.Name, req.Image)

	if err := tartClone(ctx, t.log.Writer(), req.Image, req.Name); err != nil {
		return nil, err
	}
	return &macvm.PullResponse{}, nil
}

func (t *ProxyService) Run(ctx context.Context, req *macvm.RunRequest) (*macvm.RunResponse, error) {
	debugx.Println("PROXY MACVM RUN INITIATED", req.Name)
	defer debugx.Println("PROXY MACVM RUN COMPLETED", req.Name)

	client, err := t.ensure(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if err := sshExec(ctx, client, strings.Join(req.Command, " "), t.log.Writer(), t.log.Writer()); err != nil {
		return nil, errorsx.Wrap(err, "macvm run failed")
	}
	return &macvm.RunResponse{}, nil
}

func (t *ProxyService) Module(ctx context.Context, req *macvm.ModuleRequest) (*macvm.ModuleResponse, error) {
	debugx.Println("PROXY MACVM MODULE INITIATED", req.Name, req.Module)
	defer debugx.Println("PROXY MACVM MODULE COMPLETED", req.Name, req.Module)

	client, err := t.ensure(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	src := fsx.LocateFirst(
		filepath.Join(t.ws.Root, t.ws.BuildDir, req.Module),
		eg.DefaultMountRoot(eg.RuntimeDirectory, req.Module),
	)
	if src == "" {
		return nil, errorsx.Errorf("compiled module %s not found on host", req.Module)
	}
	if err := fsx.Clone(ctx, src, filepath.Join(t.ws.RuntimeDir, eg.ModuleBin)); err != nil {
		return nil, errorsx.Wrapf(err, "stage module %s", req.Module)
	}
	if err := sshExec(ctx, client, moduleCmd(t.egbin, req.Mdir), t.log.Writer(), t.log.Writer()); err != nil {
		return nil, errorsx.Wrap(err, "macvm module failed")
	}
	return &macvm.ModuleResponse{}, nil
}

// tartClone is a no-op when a local VM named `name` already exists; cloning
// from a remote `:tag` source re-validates against the registry and re-pulls
// the full image, which is prohibitively slow for a 24GB macOS image.
// TART_NO_AUTO_PRUNE keeps the image cache around so the next clone is COW
// rather than another full pull when disk pressure rises.
func tartClone(ctx context.Context, out io.Writer, source, name string) error {
	if exec.CommandContext(ctx, "tart", "get", name).Run() == nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "tart", "clone", source, name)
	cmd.Env = append(os.Environ(), "TART_NO_AUTO_PRUNE=1")
	cmd.Stdout, cmd.Stderr = out, out
	return errorsx.Wrapf(cmd.Run(), "tart clone %s %s", source, name)
}

// ensure starts the Tart VM if not already running and returns a live ssh
// client. The VM subprocess is intentionally detached from ctx (plain
// exec.Command, not CommandContext) so a cancelled workload ctx leaves the
// guest running for the shutdown path to stop it cleanly.
func (t *ProxyService) ensure(ctx context.Context, name string) (*ssh.Client, error) {
	vmsmu.Lock()
	defer vmsmu.Unlock()

	if _, ok := vms[name]; !ok {
		run := exec.Command(
			"tart", "run", "--net-softnet",
			"--dir="+shareWorking+":"+t.ws.WorkingDir,
			"--dir="+shareCache+":"+t.ws.CacheDir,
			"--dir="+shareRuntime+":"+t.ws.RuntimeDir,
			"--dir="+shareWorkspace+":"+t.ws.WorkspaceDir,
			"--dir="+shareEGBin+":"+filepath.Dir(t.egbin),
			name,
		)
		run.Env = append(os.Environ(), "TART_NO_AUTO_PRUNE=1")
		run.Stdout, run.Stderr = t.log.Writer(), t.log.Writer()
		if err := run.Start(); err != nil {
			return nil, errorsx.Wrapf(err, "tart run %s", name)
		}
		vms[name] = run
	}

	ip, err := waitForGuestIP(ctx, name, 2*time.Minute)
	if err != nil {
		return nil, err
	}
	client, err := dialGuestSSH(ctx, ip, 2*time.Minute)
	if err != nil {
		return nil, err
	}
	if err := configureGuestSudoers(ctx, client); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

// configureGuestSudoers puts /opt/homebrew/bin on sudo's secure_path so the
// shell.Op pipeline (sudo -E -H -u admin -g staff bash -c …) can find brew,
// go, flutter etc. Cirrus images ship with NOPASSWD admin so writing via
// `sudo tee` succeeds without prompting.
func configureGuestSudoers(ctx context.Context, client *ssh.Client) error {
	const cmd = `sudo tee /etc/sudoers.d/eg-secure-path >/dev/null <<'EOF' && sudo chmod 0440 /etc/sudoers.d/eg-secure-path
Defaults secure_path = "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
Defaults env_keep += "PATH"
EOF`
	return errorsx.Wrap(sshExec(ctx, client, cmd, io.Discard, io.Discard), "configure guest sudoers")
}

func waitForGuestIP(ctx context.Context, name string, timeout time.Duration) (string, error) {
	cmd := exec.CommandContext(ctx, "tart", "ip", "--wait", fmt.Sprintf("%d", int(timeout.Seconds())), name)
	out, err := cmd.Output()
	if err != nil {
		return "", errorsx.Wrapf(err, "tart ip --wait %s", name)
	}
	return strings.TrimSpace(string(out)), nil
}

func dialGuestSSH(ctx context.Context, ip string, timeout time.Duration) (*ssh.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cfg := &ssh.ClientConfig{
		User:            guestSSHUser,
		Auth:            []ssh.AuthMethod{ssh.Password(guestSSHPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	for {
		client, err := ssh.Dial("tcp", net.JoinHostPort(ip, "22"), cfg)
		if err == nil {
			return client, nil
		}
		select {
		case <-ctx.Done():
			return nil, errorsx.Wrapf(err, "guest sshd at %s never answered", ip)
		case <-time.After(2 * time.Second):
		}
	}
}

func sshExec(ctx context.Context, client *ssh.Client, cmd string, stdout, stderr io.Writer) error {
	session, err := client.NewSession()
	if err != nil {
		return errorsx.Wrap(err, "open ssh session")
	}
	defer session.Close()

	session.Stdout, session.Stderr = stdout, stderr
	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// moduleCmd builds the in-guest `eg module` invocation. virtio-fs cannot host
// AF_UNIX sockets so the in-guest runtime dir lives on /tmp; everything else
// resolves through the virtio-fs shares under guestSharesMount.
func moduleCmd(egbin, mdir string) string {
	egbinDir := filepath.Join(guestSharesMount, shareEGBin)
	workingDir := filepath.Join(guestSharesMount, shareWorking)
	cacheDir := filepath.Join(guestSharesMount, shareCache)
	workspaceDir := filepath.Join(guestSharesMount, shareWorkspace)
	runtimeShare := filepath.Join(guestSharesMount, shareRuntime)
	guestRuntimeDir := "/tmp/eg-mvm-rt"
	return fmt.Sprintf(
		"rm -rf %s && mkdir -p %s && env DYLD_LIBRARY_PATH=%s %s=%s %s=%s %s=%s %s=%s %s module --directory %s --runtimedir %s --moduledir %s %s",
		shquote(guestRuntimeDir),
		shquote(guestRuntimeDir),
		shquote(egbinDir),
		eg.EnvComputeWorkingDirectory, shquote(workingDir),
		eg.EnvComputeRuntimeDirectory, shquote(guestRuntimeDir),
		eg.EnvComputeCacheDirectory, shquote(cacheDir),
		eg.EnvComputeWorkloadDirectory, shquote(workspaceDir),
		shquote(filepath.Join(egbinDir, filepath.Base(egbin))),
		shquote(workingDir),
		shquote(guestRuntimeDir),
		shquote(mdir),
		shquote(filepath.Join(runtimeShare, eg.ModuleBin)),
	)
}

func shquote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shutdown stops every VM the proxy started. Tart's clone state survives
// `tart stop`, so the next Pull short-circuits on the existing VM and the
// brew/build caches inside it carry across runs.
func shutdown(ctx context.Context) error {
	vmsmu.Lock()
	defer vmsmu.Unlock()

	var stopErr error
	for name, cmd := range vms {
		if err := exec.CommandContext(ctx, "tart", "stop", name).Run(); err != nil {
			stopErr = errorsx.Compact(stopErr, errorsx.Wrapf(err, "tart stop %s", name))
		}
		_ = cmd.Wait()
		delete(vms, name)
	}
	return stopErr
}
