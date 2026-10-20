package cmdmachine

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Cmd struct {
	SSH SSH `cmd:"" name:"ssh" help:"ssh into a machine started with 'eg compute local --ssh' using eg's managed ssh identity"`
}

type SSH struct {
	SSHKeyPath string        `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Host       string        `name:"host" help:"host of the machine" default:"localhost"`
	Port       int           `name:"port" help:"port the machine's ssh server is published on" default:"2222"`
	User       string        `name:"user" help:"user to connect as" default:"${vars_machine_ssh_user}"`
	Timeout    time.Duration `name:"timeout" help:"maximum time to wait when connecting" default:"10s"`
}

func (t SSH) Run(gctx *cmdopts.Global) (err error) {
	var (
		signer ssh.Signer
		client *ssh.Client
		pkey   []byte
	)

	if pkey, err = os.ReadFile(t.SSHKeyPath); err != nil {
		return errorsx.Wrapf(err, "unable to read ssh key %s, generate it with 'eg ssh key'", t.SSHKeyPath)
	}

	if signer, err = ssh.ParsePrivateKey(pkey); err != nil {
		return errorsx.Wrapf(err, "unable to parse ssh key %s", t.SSHKeyPath)
	}

	cfg := &ssh.ClientConfig{
		User: t.User,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		// machines are ephemeral and generate a new host key on every start,
		// so there is nothing meaningful to verify (known_hosts is not consulted).
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         t.Timeout,
	}

	addr := net.JoinHostPort(t.Host, fmt.Sprint(t.Port))
	if client, err = ssh.Dial("tcp", addr, cfg); err != nil {
		return errorsx.Wrapf(err, "unable to connect to %s", addr)
	}
	defer client.Close()

	return shell(gctx.Context, client)
}

func shell(ctx context.Context, client *ssh.Client) (err error) {
	session, err := client.NewSession()
	if err != nil {
		return errorsx.Wrap(err, "unable to create ssh session")
	}
	defer session.Close()

	// unblock the session when the process is cancelled.
	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return errorsx.New("an interactive shell requires a terminal")
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return errorsx.Wrap(err, "unable to configure terminal")
	}
	defer term.Restore(fd, state)

	width, height, err := term.GetSize(fd)
	if err != nil {
		return errorsx.Wrap(err, "unable to determine terminal size")
	}

	if err = session.RequestPty(os.Getenv("TERM"), height, width, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		return errorsx.Wrap(err, "unable to request pty")
	}

	defer watchresize(ctx, fd, session)()

	if err = session.Shell(); err != nil {
		return errorsx.Wrap(err, "unable to start shell")
	}

	return session.Wait()
}
