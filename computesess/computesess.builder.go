package computesess

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/uuidx"
)

// Builder describes a run to register in the cli's database, see Register.
type Builder struct {
	db        *sql.DB
	sshport   int
	sshkey    string
	capagent  bool
	cappatch  bool
	ptysocket string
	branches  []string
	container func(uuidx.UUID) string
}

// New creates a Builder registering in db, a nil db, i.e. the cli's database is unavailable, makes
// registering a no-op.
func New(db *sql.DB) Builder {
	return Builder{db: db, sshport: -1, container: func(id uuidx.UUID) string { return fmt.Sprintf("eg-%s", id) }}
}

// SSH marks the run as reachable over ssh. port is the --ssh flag the run was started with: negative
// disables ssh, 0 lets podman pick the port, otherwise the port. the port the container actually
// published is waited for when registering.
func (t Builder) SSH(port int, keypath string) Builder {
	t.sshport = port
	t.sshkey = keypath
	return t
}

// Agent marks the run as being able to host an agent.
func (t Builder) Agent(b bool) Builder {
	t.capagent = b
	return t
}

// Branch chooses the branch the run's changes are committed to, so the run can be resumed on it. the first name
// is the one requested, it wins when it is not blank. otherwise the branch the run saved when it last ran is used,
// and when it saved none the first of the remaining names that is not blank, which are the defaults.
func (t Builder) Branch(names ...string) Builder {
	t.branches = names
	return t
}

// PTY records the unix socket the run's terminal can be attached to, see ttyx.Host. blank, the default, means the run has
// no terminal to attach to.
func (t Builder) PTY(socket string) Builder {
	t.ptysocket = socket
	return t
}

// Patch marks the run's changes as being patched back to the repository.
func (t Builder) Patch(b bool) Builder {
	t.cappatch = b
	return t
}

// Register the run as running so tools, i.e. the editor extension, can discover it. the session returned is
// the one registered, its branch is the one requested or else the one the run saved when it last ran, blank when
// there is none. when ssh is enabled the registration waits, in the background, for the container (named
// eg-<id>) to publish its port, so this should be called before the container is started. the returned function
// ends the registration: it stops waiting, marks the run as stopped and waits for that to complete, cause is
// the reason the run ended and may be nil. it must be called exactly once.
func (t Builder) Register(ctx context.Context, id uuidx.UUID, module string, dir string) (_ Session, done context.CancelCauseFunc) {
	sshon := t.sshport >= 0
	s := Session{
		ID:           id.String(),
		Module:       module,
		Directory:    dir,
		SSHRequested: int64(t.sshport),
		SSHUser:      eg.DefaultUsername,
		SSHKeyPath:   t.sshkey,
		CapSSH:       sshon,
		CapAgent:     t.capagent,
		CapPatch:     t.cappatch,
		PtySocket:    t.ptysocket,
	}

	if t.db == nil {
		s.Branch = t.branch(ctx, s.ID)
		return s, func(error) {}
	}

	s.Branch = t.branch(ctx, s.ID)

	var (
		started      = time.Now()
		wctx, cancel = context.WithCancelCause(ctx)
		finished     = make(chan struct{})
		registering  = s // owned by the registration, s is returned to the caller.
	)

	register := func() bool {
		if err := Register(wctx, t.db, registering); err != nil {
			log.Println("unable to register serve session", err)
			return false
		}
		return true
	}

	if !sshon {
		register()
		close(finished)
	} else {
		go func() {
			defer close(finished)

			if registering.SSHPort = WaitSSHPort(wctx, t.container(id), started); registering.SSHPort == 0 {
				return
			}

			if !register() {
				return
			}

			log.Printf("ssh access enabled: ssh -i %s -p %d -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o LogLevel=ERROR %s@localhost\n", registering.SSHKeyPath, registering.SSHPort, registering.SSHUser)
		}()
	}

	return s, func(cause error) {
		cancel(cause)
		<-finished

		// the run's context may already be cancelled so stopping uses its own.
		if err := Stop(context.Background(), t.db, s.ID); err != nil {
			log.Println(err)
		}
	}
}

// branch resolves the branch of the run, see Branch. the saved branch is only looked up when none was requested.
func (t Builder) branch(ctx context.Context, id string) string {
	var requested string
	var defaults []string
	if len(t.branches) > 0 {
		requested, defaults = t.branches[0], t.branches[1:]
	}

	if stringsx.Present(requested) {
		return requested
	}

	var saved string
	if t.db != nil {
		if found, err := Lookup(ctx, t.db, id); err == nil {
			saved = found.Branch
		}
	}

	return stringsx.FirstNonBlank(append([]string{saved}, defaults...)...)
}
