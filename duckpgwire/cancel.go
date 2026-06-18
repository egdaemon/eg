package duckpgwire

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
)

// cancelKey identifies a session for purposes of the Postgres
// CancelRequest message, which arrives on a throwaway connection
// carrying the (ProcessID, SecretKey) pair handed out at startup.
type cancelKey struct {
	pid    uint32
	secret uint32
}

type cancelRegistry struct {
	mu       sync.Mutex
	sessions map[uint32]cancelEntry
	nextPID  uint32
}

type cancelEntry struct {
	secret uint32
	cancel func()
}

func newCancelRegistry() *cancelRegistry {
	return &cancelRegistry{sessions: make(map[uint32]cancelEntry)}
}

// register allocates a new (pid, secret) pair for a session and stores its
// cancel func, returning the pair to send in BackendKeyData.
func (r *cancelRegistry) register(cancel func()) (pid, secret uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextPID++
	pid = r.nextPID
	secret = randUint32()
	r.sessions[pid] = cancelEntry{secret: secret, cancel: cancel}
	return pid, secret
}

func (r *cancelRegistry) deregister(pid uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, pid)
}

// cancel invokes the cancel func registered for (pid, secret) if the secret
// matches, mirroring Postgres's CancelRequest authorization check.
func (r *cancelRegistry) cancel(pid, secret uint32) {
	r.mu.Lock()
	entry, ok := r.sessions[pid]
	r.mu.Unlock()

	if ok && entry.secret == secret {
		entry.cancel()
	}
}

func randUint32() uint32 {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint32(buf[:])
}
