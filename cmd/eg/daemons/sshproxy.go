package daemons

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func SSHProxy(global *cmdopts.Global, config *ssh.ClientConfig, signer ssh.Signer, httpl net.Listener) (proxyl net.Listener, err error) {
	// TODO: use a tls dialer so we can proxy through 443 based on the alpn id.
	conn, err := ssh.Dial("tcp", "localhost:8090", config)
	if err != nil {
		return nil, errors.Wrap(err, "unable to listen for ssh connections")
	}
	defer conn.Close()

	if proxyl, err = conn.Listen("tcp", "127.0.0.1:0"); err != nil {
		return nil, errors.Wrap(err, "unable to listen for ssh connections")
	}

	// proxyl, err := conn.Listen("unix", "derp.socket")
	// if err != nil {
	// 	log.Fatal("unable to listen to unix connection: ", err)
	// }

	global.Cleanup.Add(1)
	go func() {
		defer global.Cleanup.Done()
		defer global.Shutdown()

		d := net.Dialer{}
		for {
			proxied, err := proxyl.Accept()
			if err != nil {
				log.Println("unable to accept new proxied connections", err)
				return
			}

			log.Println("proxying requested", proxied.LocalAddr().String(), proxied.RemoteAddr().String())
			forward(global.Context, &d, proxied)
			log.Println("proxying initiated", proxied.LocalAddr().String(), proxied.RemoteAddr().String())
		}
	}()

	return proxyl, nil
}

type dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func forward(ctx context.Context, d dialer, proxied net.Conn) {
	cleanup := func() {
		errorsx.MaybeLog(errors.Wrap(proxied.Close(), "failed to close proxy connection"))
	}

	dconn, err := d.DialContext(ctx, "tcp", "127.0.1.1:8093")
	if err != nil {
		log.Println("unable to establish connection", err)
		cleanup()
		return
	}

	// Copy localConn.Reader to sshConn.Writer
	go func() {
		defer log.Println("connection shutting down")
		defer cleanup()

		if _, err := io.Copy(dconn, proxied); err != nil {
			log.Println("copy failed", err)
		}
	}()

	// Copy sshConn.Reader to localConn.Writer
	go func() {
		defer log.Println("connection shutting down")
		defer cleanup()
		if _, err = io.Copy(proxied, dconn); err != nil {
			log.Println("copy failed", err)
		}
	}()
}
