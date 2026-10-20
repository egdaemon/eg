package cmdmachine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// serve accepts a single connection, authenticating only the authorized key.
func serve(t *testing.T, authorized ssh.PublicKey) (addr string) {
	_, hostpriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hostsigner, err := ssh.NewSignerFromKey(hostpriv)
	require.NoError(t, err)

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(authorized.Marshal()) {
				return nil, nil
			}
			return nil, os.ErrPermission
		},
	}
	cfg.AddHostKey(hostsigner)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
		if err != nil {
			return
		}
		defer conn.Close()
		go ssh.DiscardRequests(reqs)

		for nc := range chans {
			ch, creqs, err := nc.Accept()
			if err != nil {
				return
			}
			go ssh.DiscardRequests(creqs)
			defer ch.Close()
		}
	}()

	return l.Addr().String()
}

func TestSSHRequiresTerminal(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	keypath := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(keypath, pem.EncodeToMemory(block), 0600))

	host, port, err := net.SplitHostPort(serve(t, signer.PublicKey()))
	require.NoError(t, err)
	p, err := net.LookupPort("tcp", port)
	require.NoError(t, err)

	// connecting and authenticating succeed, the shell is then refused since tests have no terminal.
	err = SSH{SSHKeyPath: keypath, Host: host, Port: p, User: "egd", Timeout: 5 * time.Second}.Run(&cmdopts.Global{Context: t.Context()})
	require.ErrorContains(t, err, "requires a terminal")
}

func TestSSHMissingKey(t *testing.T) {
	err := SSH{SSHKeyPath: filepath.Join(t.TempDir(), "missing")}.Run(&cmdopts.Global{Context: t.Context()})
	require.Error(t, err)
}
