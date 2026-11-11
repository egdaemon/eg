package memlimitx_test

import (
	"errors"
	"math"
	"os"
	"runtime/debug"
	"testing"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/egdaemon/eg/internal/memlimitx"
	"github.com/stretchr/testify/require"
)

func TestFromEnviron(t *testing.T) {
	t.Run("first key to parse wins", func(t *testing.T) {
		t.Setenv("MEMLIMITX_TEST_A", "garbage")
		t.Setenv("MEMLIMITX_TEST_B", "2GiB")
		t.Setenv("MEMLIMITX_TEST_C", "4GiB")

		limit, err := memlimitx.FromEnviron("MEMLIMITX_TEST_UNSET", "MEMLIMITX_TEST_A", "MEMLIMITX_TEST_B", "MEMLIMITX_TEST_C")()
		require.NoError(t, err)
		require.Equal(t, uint64(2*1024*1024*1024), limit)
	})

	t.Run("plain integer", func(t *testing.T) {
		t.Setenv("MEMLIMITX_TEST_A", "1048576")

		limit, err := memlimitx.FromEnviron("MEMLIMITX_TEST_A")()
		require.NoError(t, err)
		require.Equal(t, uint64(1048576), limit)
	})

	t.Run("no limit when unset", func(t *testing.T) {
		_, err := memlimitx.FromEnviron("MEMLIMITX_TEST_UNSET")()
		require.ErrorIs(t, err, memlimit.ErrNoLimit)
	})
}

func TestChain(t *testing.T) {
	nolimit := func() (uint64, error) { return 0, memlimit.ErrNoLimit }

	t.Run("first provider with a limit wins", func(t *testing.T) {
		t.Setenv("AUTOMEMLIMIT", "1")
		defer debug.SetMemoryLimit(debug.SetMemoryLimit(-1))
		os.Unsetenv("GOMEMLIMIT")

		limit, err := memlimit.Set(memlimitx.Chain(nolimit, memlimit.Limit(2048), memlimit.Limit(4096)))
		require.NoError(t, err)
		require.Equal(t, int64(2048), limit)
	})

	t.Run("errors that are not ErrNoLimit stop the chain", func(t *testing.T) {
		t.Setenv("AUTOMEMLIMIT", "1")
		defer debug.SetMemoryLimit(debug.SetMemoryLimit(-1))
		os.Unsetenv("GOMEMLIMIT")

		boom := errors.New("boom")
		_, err := memlimit.Set(memlimitx.Chain(func() (uint64, error) { return 0, boom }, memlimit.Limit(4096)))
		require.ErrorIs(t, err, boom)
	})

	t.Run("no limit when every provider reports none", func(t *testing.T) {
		t.Setenv("AUTOMEMLIMIT", "1")
		defer debug.SetMemoryLimit(debug.SetMemoryLimit(-1))
		os.Unsetenv("GOMEMLIMIT")

		limit, err := memlimit.Set(memlimitx.Chain(nolimit, nolimit))
		require.NoError(t, err)
		require.Equal(t, int64(math.MaxInt64), limit)
	})
}
