package egsystemd_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/egdaemon/eg/runtime/x/wasi/egsystemd"
	"github.com/stretchr/testify/require"
)

// mockConn implements the dbusConn interface for testing.
type mockConn struct {
	mu       sync.Mutex
	units    []dbus.UnitStatus
	listErr  error
	updateCh chan<- *dbus.SubStateUpdate
	errCh    chan<- error
}

func (m *mockConn) ListUnitsByNamesContext(_ context.Context, _ []string) ([]dbus.UnitStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.units, m.listErr
}

func (m *mockConn) SetSubStateSubscriber(updateCh chan<- *dbus.SubStateUpdate, errCh chan<- error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCh = updateCh
	m.errCh = errCh
}

func (m *mockConn) Close() {}

func (m *mockConn) RestartUnitContext(_ context.Context, _, _ string, _ chan<- string) (int, error) {
	return 0, nil
}

func (m *mockConn) ReloadUnitContext(_ context.Context, _, _ string, _ chan<- string) (int, error) {
	return 0, nil
}

func (m *mockConn) ReloadOrRestartUnitContext(_ context.Context, _, _ string, _ chan<- string) (int, error) {
	return 0, nil
}

func connFn(conn egsystemd.Conn) egsystemd.ConnectionFn {
	return func(_ context.Context) (egsystemd.Conn, error) {
		return conn, nil
	}
}

func (m *mockConn) sendUpdate(unit, substate string) {
	m.mu.Lock()
	ch := m.updateCh
	m.mu.Unlock()
	ch <- &dbus.SubStateUpdate{UnitName: unit, SubState: substate}
}

func (m *mockConn) sendErr(err error) {
	m.mu.Lock()
	ch := m.errCh
	m.mu.Unlock()
	ch <- err
}

func (m *mockConn) setUnits(units []dbus.UnitStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.units = units
}

func activeUnits(names ...string) []dbus.UnitStatus {
	out := make([]dbus.UnitStatus, len(names))
	for i, n := range names {
		out[i] = dbus.UnitStatus{Name: n, ActiveState: "active", SubState: "running"}
	}
	return out
}

func inactiveUnit(name string) dbus.UnitStatus {
	return dbus.UnitStatus{Name: name, ActiveState: "failed", SubState: "failed"}
}

func TestEnsureRunning(t *testing.T) {
	t.Run("returns_error_when_unit_not_active_on_initial_check", func(t *testing.T) {
		conn := &mockConn{
			units: []dbus.UnitStatus{inactiveUnit("foo.service")},
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.Error(t, err)
		require.Contains(t, err.Error(), "foo.service")
	})

	t.Run("returns_context_error_when_cancelled", func(t *testing.T) {
		conn := &mockConn{
			units: activeUnits("foo.service"),
		}
		ctx, cancel := context.WithCancel(t.Context())

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns_error_when_unit_fails_after_update", func(t *testing.T) {
		conn := &mockConn{
			units: activeUnits("foo.service"),
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go func() {
			time.Sleep(20 * time.Millisecond)
			conn.setUnits([]dbus.UnitStatus{inactiveUnit("foo.service")})
			conn.sendUpdate("foo.service", "failed")
		}()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.Error(t, err)
		require.Contains(t, err.Error(), "foo.service")
	})

	t.Run("ignores_updates_for_unknown_units", func(t *testing.T) {
		conn := &mockConn{
			units: activeUnits("foo.service"),
		}
		ctx, cancel := context.WithCancel(t.Context())

		go func() {
			time.Sleep(20 * time.Millisecond)
			conn.sendUpdate("unrelated.service", "running")
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("propagates_error_from_dbus_error_channel", func(t *testing.T) {
		conn := &mockConn{
			units: activeUnits("foo.service"),
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		sentinel := errors.New("dbus subscription error")
		go func() {
			time.Sleep(20 * time.Millisecond)
			conn.sendErr(sentinel)
		}()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("deadline_exceeded_final_check_passes_returns_nil", func(t *testing.T) {
		conn := &mockConn{
			units: activeUnits("foo.service"),
		}
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.NoError(t, err)
	})

	t.Run("deadline_exceeded_final_check_fails_returns_error", func(t *testing.T) {
		conn := &mockConn{
			units: activeUnits("foo.service"),
		}
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()

		go func() {
			time.Sleep(25 * time.Millisecond)
			conn.setUnits([]dbus.UnitStatus{inactiveUnit("foo.service")})
		}()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.Error(t, err)
		require.Contains(t, err.Error(), "foo.service")
	})

	t.Run("list_units_error_on_initial_check_returns_error", func(t *testing.T) {
		sentinel := errors.New("dbus list error")
		conn := &mockConn{
			listErr: sentinel,
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := egsystemd.EnsureRunning(ctx, connFn(conn), "foo.service")
		require.ErrorIs(t, err, sentinel)
	})
}
