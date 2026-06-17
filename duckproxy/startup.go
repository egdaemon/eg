package duckproxy

import (
	"encoding/binary"

	"github.com/jackc/pgx/v5/pgproto3"
)

// negotiateStartup performs the pre-authentication handshake: denying
// SSL/GSS encryption requests (this proxy is unix-socket-only, encryption
// is meaningless here), answering CancelRequest out-of-band, and waiting
// for the real StartupMessage. proceed is false if the connection should
// simply be closed without further processing -- a CancelRequest was
// handled (it arrives on its own throwaway connection per the protocol
// spec and expects no reply), or the peer disconnected after an SSL/GSS
// deny.
func (s *session) negotiateStartup() (proceed bool, err error) {
	for {
		msg, err := s.backend.ReceiveStartupMessage()
		if err != nil {
			return false, err
		}

		switch m := msg.(type) {
		case *pgproto3.SSLRequest:
			if _, err := s.conn.Write([]byte{'N'}); err != nil {
				return false, err
			}
		case *pgproto3.GSSEncRequest:
			if _, err := s.conn.Write([]byte{'N'}); err != nil {
				return false, err
			}
		case *pgproto3.CancelRequest:
			var secret uint32
			if len(m.SecretKey) >= 4 {
				secret = binary.BigEndian.Uint32(m.SecretKey)
			}
			s.server.registry.cancel(m.ProcessID, secret)
			return false, nil
		case *pgproto3.StartupMessage:
			return true, nil
		}
	}
}

// sendHandshake replies to a StartupMessage with AuthenticationOk (trust
// auth -- access control is the unix socket's file permissions, nothing
// more), a fixed ParameterStatus set, BackendKeyData, and an initial
// ReadyForQuery.
func (s *session) sendHandshake() error {
	secretKey := make([]byte, 4)
	binary.BigEndian.PutUint32(secretKey, s.secret)

	s.backend.Send(&pgproto3.AuthenticationOk{})
	s.backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "14.0"})
	s.backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	s.backend.Send(&pgproto3.ParameterStatus{Name: "server_encoding", Value: "UTF8"})
	s.backend.Send(&pgproto3.ParameterStatus{Name: "DateStyle", Value: "ISO, MDY"})
	s.backend.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	s.backend.Send(&pgproto3.BackendKeyData{ProcessID: s.pid, SecretKey: secretKey})
	s.backend.Send(&pgproto3.ReadyForQuery{TxStatus: s.tx.statusByte()})
	return s.backend.Flush()
}
