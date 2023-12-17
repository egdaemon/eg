package daemons

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/iox"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func SSHProxy(global *cmdopts.Global, config *ssh.ClientConfig, signer ssh.Signer, httpl net.Listener) (proxyl net.Listener, err error) {
	// TODO: use a tls dialer so we can proxy through 443 based on the alpn id.
	conn, err := ssh.Dial("tcp", "localhost:8090", config)
	if err != nil {
		return nil, errors.Wrap(err, "unable to listen for ssh connections")
	}

	if proxyl, err = conn.Listen("tcp", "127.0.0.1:0"); err != nil {
		return nil, errors.Wrap(err, "unable to listen for proxied http connections")
	}

	// if proxyl, err = conn.Listen("unix", "derp.socket"); err != nil {
	// 	return nil, errors.Wrap(err, "unable to listen for ssh connections")
	// }

	log.Println("PROXY", proxyl.Addr().Network(), proxyl.Addr().String())
	global.Cleanup.Add(1)
	go func() {
		defer conn.Close()
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
			forward(global.Context, httpl, &d, proxied)
			log.Println("proxying initiated", proxied.LocalAddr().String(), proxied.RemoteAddr().String())
		}
	}()

	return proxyl, nil
}

type dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func forward(ctx context.Context, dst net.Listener, d dialer, proxied net.Conn) {
	cleanup := func() {
		errorsx.MaybeLog(errors.Wrap(iox.IgnoreEOF(proxied.Close()), "failed to close proxy connection"))
	}

	dconn, err := d.DialContext(ctx, dst.Addr().Network(), dst.Addr().String())
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
