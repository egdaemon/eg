package ttyx

import (
	"log"
	"os"
	"sync"

	xterm "golang.org/x/term"
)

// raw is the hold on one terminal device. the device's mode is shared by everything that has it open, so the first
// hold records the mode it is restored to and the last hold to be released restores it. restoring per caller
// instead leaves the terminal raw whenever two overlapping holds are released in the order they were taken.
type raw struct {
	refs  int
	ctl   *os.File // a descriptor of our own for the device, the caller's may be closed before the last release.
	owned bool     // ctl is a duplicate to close once the device is restored.
	state *xterm.State
}

var (
	rawmu sync.Mutex
	raws  = map[string]*raw{}
)

// Raw puts the terminal f into raw mode for input, key presses are delivered one at a time without echo or signals,
// while leaving output processing on: every \n written to the terminal by anyone still returns to the first column.
// it is safe to call from many goroutines on the same terminal, the terminal is restored when every returned
// function has been called.
func Raw(f *os.File) (restore func(), err error) {
	return acquire(f, false)
}

// cbreak is Raw except the terminal keeps generating signals, i.e.) ctrl-c still interrupts the process group.
func cbreak(f *os.File) (restore func(), err error) {
	return acquire(f, true)
}

func acquire(f *os.File, keepsig bool) (restore func(), err error) {
	key, err := devkey(f)
	if err != nil {
		return nil, err
	}

	rawmu.Lock()
	defer rawmu.Unlock()

	if r, ok := raws[key]; ok {
		r.refs++
		return release(key, r), nil
	}

	r := &raw{refs: 1, ctl: f}
	if d, err := dup(f); err == nil {
		r.ctl, r.owned = d, true
	}

	fd := int(r.ctl.Fd())
	if r.state, err = xterm.MakeRaw(fd); err != nil {
		r.close()
		return nil, err
	}

	if err = cooked(fd, keepsig); err != nil {
		_ = xterm.Restore(fd, r.state)
		r.close()
		return nil, err
	}

	raws[key] = r
	return release(key, r), nil
}

func release(key string, r *raw) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			rawmu.Lock()
			defer rawmu.Unlock()

			if r.refs--; r.refs > 0 {
				return
			}

			delete(raws, key)
			if err := xterm.Restore(int(r.ctl.Fd()), r.state); err != nil {
				log.Println("unable to restore terminal state", err)
			}
			r.close()
		})
	}
}

func (t *raw) close() {
	if t.owned {
		_ = t.ctl.Close()
	}
}
