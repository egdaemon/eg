package egsystemd

import (
	"context"
	"log"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/pkg/errors"
)

// PerformFn is the signature shared by dbus job-dispatch methods such as
// RestartUnitContext, ReloadUnitContext, and ReloadOrRestartUnitContext.
// It submits a job for the named unit using the given mode (e.g. "replace")
// and delivers the job result string (e.g. "done", "failed") to ch when the
// job completes.
type PerformFn func(context.Context, string, string, chan<- string) (int, error)

func resultToError(result string) error {
	if result == "done" {
		return nil
	}
	return errors.New(result)
}

func startJob(ctx context.Context, d PerformFn, targets ...string) error {
	await := make(chan string)
	defer close(await)

	for _, target := range targets {
		_, err := d(ctx, target, "replace", await)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-await:
			if err := resultToError(result); err != nil {
				return err
			}
		}
	}

	return nil
}

// Conn is the D-Bus connection interface used by all functions in this package.
// *dbus.Conn satisfies this interface; in tests a mock implementation can be
// returned from a ConnectionFn without requiring a live D-Bus connection.
type Conn interface {
	Close()
	ListUnitsByNamesContext(ctx context.Context, units []string) ([]dbus.UnitStatus, error)
	SetSubStateSubscriber(updateCh chan<- *dbus.SubStateUpdate, errCh chan<- error)
	RestartUnitContext(ctx context.Context, name, mode string, ch chan<- string) (int, error)
	ReloadUnitContext(ctx context.Context, name, mode string, ch chan<- string) (int, error)
	ReloadOrRestartUnitContext(ctx context.Context, name, mode string, ch chan<- string) (int, error)
}

// ConnectionFn opens a D-Bus connection to systemd. Pass SystemBus or UserBus
// to target the system or per-user instance respectively, or supply a custom
// implementation for testing.
type ConnectionFn func(ctx context.Context) (Conn, error)

// SystemBus opens a D-Bus connection to the system-wide systemd instance.
func SystemBus(ctx context.Context) (Conn, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	return conn, errors.Wrap(err, "failed to connect to systemd bus")
}

// UserBus opens a D-Bus connection to the per-user systemd instance.
func UserBus(ctx context.Context) (Conn, error) {
	conn, err := dbus.NewUserConnectionContext(ctx)
	return conn, errors.Wrap(err, "failed to connect to systemd bus")
}

// Restart issues a restart job for each unit in sequence, waiting for each
// job to complete before moving to the next. cfn is called once to obtain
// the D-Bus connection; pass SystemBus or UserBus as appropriate.
func Restart(ctx context.Context, cfn ConnectionFn, units ...string) (err error) {
	var (
		conn Conn
	)

	if conn, err = cfn(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to systemd bus")
	}
	defer conn.Close()

	return errors.Wrap(startJob(ctx, conn.RestartUnitContext, units...), "systemd start unit failed")
}

// Reload issues a reload job for each unit in sequence, waiting for each job
// to complete before moving to the next. Use this when the unit supports
// reloading its configuration without a full restart. cfn is called once to
// obtain the D-Bus connection; pass SystemBus or UserBus as appropriate.
func Reload(ctx context.Context, cfn ConnectionFn, units ...string) (err error) {
	var (
		conn Conn
	)

	if conn, err = cfn(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to systemd bus")
	}
	defer conn.Close()

	return errors.Wrap(startJob(ctx, conn.ReloadUnitContext, units...), "systemd start unit failed")
}

// ReloadOrRestart issues a reload-or-restart job for each unit in sequence,
// waiting for each job to complete before moving to the next. systemd reloads
// the unit if it supports reloading, otherwise it restarts it. cfn is called
// once to obtain the D-Bus connection; pass SystemBus or UserBus as
// appropriate.
func ReloadOrRestart(ctx context.Context, cfn ConnectionFn, units ...string) (err error) {
	var (
		conn Conn
	)

	if conn, err = cfn(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to systemd bus")
	}
	defer conn.Close()

	return errors.Wrap(startJob(ctx, conn.ReloadOrRestartUnitContext, units...), "systemd start unit failed")
}

// EnsureRunning blocks until all of the named systemd units are in the
// "active" state, the context is cancelled, or an error is detected.
//
// It subscribes to systemd sub-state change events so it can react
// promptly when a unit transitions state rather than polling on a fixed
// interval.  On every relevant event it re-queries the live state of all
// requested units; if any unit is not "active" it returns a descriptive
// error.
//
// Special deadline behaviour: when the context expires with
// context.DeadlineExceeded the function performs one final check (with a
// fresh 3-second timeout) before returning.  This lets a caller impose a
// hard wall-clock limit on startup while still tolerating a unit that
// becomes active right at the deadline.
//
// Any other context cancellation is returned as-is (wrapped with a stack
// trace).
func EnsureRunning(ctx context.Context, cfn ConnectionFn, units ...string) (err error) {
	conn, err := cfn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to systemd bus")
	}
	defer conn.Close()
	return ensureRunning(ctx, conn, units...)
}

func ensureRunning(ctx context.Context, conn Conn, units ...string) (err error) {
	var (
		updates = make(chan *dbus.SubStateUpdate, 100)
		errs    = make(chan error, 100)
	)
	defer close(updates)

	detectFailed := func(ctx context.Context) error {
		upds, cause := conn.ListUnitsByNamesContext(ctx, units)
		if cause != nil {
			return errors.Wrap(cause, "unable to determine unit states")
		}

		for _, u := range upds {
			debugx.Printf("detecting unit state %+v\n", u)
			switch u.ActiveState {
			case "active":
			default:
				return errors.Errorf("%s %s - %s", u.Name, u.ActiveState, u.SubState)
			}
		}

		return nil
	}
	conn.SetSubStateSubscriber(updates, errs)
	defer conn.SetSubStateSubscriber(nil, nil)

	debugx.Println("monitoring units initiated", units)
	defer debugx.Println("monitoring units completed", units)

	if err = detectFailed(ctx); err != nil {
		return err
	}

	m := make(map[string]struct{}, len(units))
	for _, u := range units {
		m[u] = struct{}{}
	}

	for {
		select {
		case <-ctx.Done():
			if x := ctx.Err(); x == context.DeadlineExceeded {
				// if the deadline passed; we still want to do the final check to ensure
				// the services are running.
				fctx, fdone := context.WithTimeout(context.Background(), 3*time.Second)
				defer fdone()
				return detectFailed(fctx)
			} else {
				return errors.WithStack(x)
			}
		case upd := <-updates:
			if upd == nil {
				continue
			}
			if _, ok := m[upd.UnitName]; !ok {
				log.Println("systemd unknown unit", upd.UnitName, m)
				continue
			}

			if err = detectFailed(ctx); err != nil {
				return err
			}
		case cause := <-errs:
			return errors.WithStack(cause)
		}
	}
}
