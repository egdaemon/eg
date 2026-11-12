package egvm

import (
	"context"
	"errors"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/sh/v3/syntax"
	"golang.org/x/crypto/ssh"
)

// NFSPort is the vsock port the host serves the machine's root on. vminit forwards the guest's NFS client to it from
// the same port on the guest's loopback.
const NFSPort = 2049

// defaultpath is the PATH of a command whose environment sets none.
const defaultpath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// key is the key a machine authorizes: the sshd@.service the generator's sockets start accepts the
// keys of the ssh.ephemeral-authorized_keys-all credential, which it reads from its own credentials
// rather than from a file in the root.
func key() (_ ssh.Signer, authorized string, err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = sshx.SignerFromGenerator(sshx.NewKeyGen()); err != nil {
		return nil, "", errorsx.Wrap(err, "unable to generate the machine's key")
	}

	return signer, string(ssh.MarshalAuthorizedKey(signer.PublicKey())), nil
}

// dialer opens a connection to the machine's sshd, which systemd-ssh-generator binds once the root's
// systemd starts its sockets.
type dialer func() (net.Conn, error)

// redial retries dial until it connects, ctx ends, or stopped closes, when it returns stop's error.
func redial(ctx context.Context, dial dialer, stopped <-chan struct{}, stop func() error) (net.Conn, error) {
	for {
		conn, err := dial()
		if err == nil {
			return conn, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-stopped:
			return nil, errorsx.Compact(errorsx.New("the machine stopped"), stop())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// commandline is req as the line sshd hands root's shell: req.Cmd with its arguments in req.Dir, or
// / when it names none, with req.Environment as its whole environment, and PATH defaulted when it
// sets none.
func commandline(req *execproxy.ExecRequest) (_ string, err error) {
	var (
		environ = req.Environment
		words   []string
		quoted  string
	)

	if !slices.ContainsFunc(environ, func(kv string) bool { return strings.HasPrefix(kv, "PATH=") }) {
		environ = append(slices.Clone(environ), "PATH="+defaultpath)
	}

	for _, w := range slices.Concat([]string{langx.FirstNonZero(req.Dir, "/")}, environ, []string{req.Cmd}, req.Arguments) {
		if quoted, err = syntax.Quote(w, syntax.LangPOSIX); err != nil {
			return "", errorsx.Wrapf(err, "unable to quote %q", w)
		}
		words = append(words, quoted)
	}

	return "cd " + words[0] + " && exec env -i " + strings.Join(words[1:], " "), nil
}

// command runs req over ssh as root on conn and waits for it, with no input and its output discarded
// unless options say otherwise. A command that exits with other than 0 returns an ExitError, one
// killed by a signal as 128 plus the signal, like a shell's. Cancelling ctx kills the command.
func command(ctx context.Context, conn net.Conn, signer ssh.Signer, req *execproxy.ExecRequest, options ...ExecOption) (err error) {
	var (
		stdio   = newstdio(options...)
		line    string
		sconn   ssh.Conn
		chans   <-chan ssh.NewChannel
		reqs    <-chan *ssh.Request
		session *ssh.Session
		exit    *ssh.ExitError
	)
	defer conn.Close()

	if line, err = commandline(req); err != nil {
		return err
	}

	// machines generate their host keys when they boot, so there is nothing to verify them against.
	if sconn, chans, reqs, err = ssh.NewClientConn(conn, "egvm", &ssh.ClientConfig{User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.InsecureIgnoreHostKey()}); err != nil {
		return errorsx.Wrap(err, "unable to reach the machine's sshd")
	}
	client := ssh.NewClient(sconn, chans, reqs)
	defer client.Close()

	if session, err = client.NewSession(); err != nil {
		return errorsx.Wrap(err, "unable to open a session")
	}
	defer session.Close()
	session.Stdin, session.Stdout, session.Stderr = stdio.stdin, stdio.stdout, stdio.stderr

	stop := context.AfterFunc(ctx, func() {
		errorsx.Log(session.Signal(ssh.SIGKILL))
		client.Close()
	})
	defer stop()

	err = session.Run(line)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.As(err, &exit) {
		return ExitError{Code: int32(exit.ExitStatus())}
	}
	return err
}
