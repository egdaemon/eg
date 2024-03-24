package daemons

import (
	"fmt"
	"log"
	"net"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func SSHAgent(gctx *cmdopts.Global, socketpath string) error {
	log.Println("ssh agent socket", socketpath)

	s, err := net.Listen("unix", socketpath)
	if err != nil {
		return errorsx.Wrap(err, "ssh agent listen failed")
	}

	go func() {
		defer s.Close()
		for {
			conn, err := s.Accept()
			if err != nil {
				errorsx.MaybeLog(errorsx.Wrap(err, "accept failed"))
				return
			}

			go func() {
				errorsx.MaybeLog(errorsx.Wrap(agent.ServeAgent(sagent{}, conn), "agent serving done"))
			}()
		}
	}()
	return nil
}

type sagent struct{}

func (t sagent) List() ([]*agent.Key, error) {
	log.Println("LIST INITIATED")
	defer log.Println("LIST COMPLETED")
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

func (t sagent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	log.Println("SIGN INITIATED")
	defer log.Println("SIGN COMPLETED")
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}
func (t sagent) Add(key agent.AddedKey) error {
	log.Println("ADD INITIATED")
	defer log.Println("ADD COMPLETED")
	return fmt.Errorf("NOT IMPLEMENTED")
}
func (t sagent) Remove(key ssh.PublicKey) error {
	log.Println("REMOVE INITIATED")
	defer log.Println("REMOVE COMPLETED")
	return fmt.Errorf("NOT IMPLEMENTED")
}
func (t sagent) RemoveAll() error {
	log.Println("REMOVE ALL INITIATED")
	defer log.Println("REMOVE ALL COMPLETED")
	return fmt.Errorf("NOT IMPLEMENTED")
}
func (t sagent) Lock(passphrase []byte) error {
	log.Println("LOCK INITIATED")
	defer log.Println("LOCK COMPLETED")
	return fmt.Errorf("NOT IMPLEMENTED")
}
func (t sagent) Unlock(passphrase []byte) error {
	log.Println("UNLOCK INITIATED")
	defer log.Println("UNLOCK COMPLETED")
	return fmt.Errorf("NOT IMPLEMENTED")
}
func (t sagent) Signers() ([]ssh.Signer, error) {
	log.Println("SIGNERS INITIATED")
	defer log.Println("SIGNERS COMPLETED")
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}
