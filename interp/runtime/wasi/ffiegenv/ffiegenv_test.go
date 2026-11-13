package ffiegenv_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegenv"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

// a wasm module that only exports a single page of memory.
var memorymodule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic, version
	0x05, 0x03, 0x01, 0x00, 0x01, // memory section: 1 memory, min 1 page
	0x07, 0x0a, 0x01, 0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00, // export "memory"
}

func TestEnviron(t *testing.T) {
	const (
		lenptr = uint32(0)
		bufptr = uint32(8)
	)

	module := func(t *testing.T) api.Module {
		rt := wazero.NewRuntime(t.Context())
		t.Cleanup(func() { rt.Close(t.Context()) })
		m, err := rt.InstantiateWithConfig(t.Context(), memorymodule, wazero.NewModuleConfig())
		require.NoError(t, err)
		return m
	}

	file := func(t *testing.T, content string) string {
		path := filepath.Join(t.TempDir(), "environ.env")
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		return path
	}

	read := func(t *testing.T, m api.Module) (uint32, string) {
		n, ok := m.Memory().ReadUint32Le(lenptr)
		require.True(t, ok)
		data, ok := m.Memory().Read(bufptr, n)
		require.True(t, ok)
		return n, string(data)
	}

	t.Run("writes the entries joined by the separator", func(t *testing.T) {
		m := module(t)
		environ := []string{"A=1", "B=two words", "C=x=y"}

		errcode := ffiegenv.Environ(file(t, strings.Join(environ, "\n")+"\n"))(t.Context(), m, math.MaxInt64, bufptr, 1024, lenptr)
		require.Zero(t, errcode)

		_, data := read(t, m)
		require.Equal(t, environ, strings.Split(data, ffiegenv.Separator))
	})

	t.Run("reports the required length when the buffer is too small", func(t *testing.T) {
		m := module(t)
		errcode := ffiegenv.Environ(file(t, "A=1\nB=2\n"))(t.Context(), m, math.MaxInt64, bufptr, 2, lenptr)
		require.Equal(t, uint32(ffierrors.ErrBufferTooSmall), errcode)

		n, ok := m.Memory().ReadUint32Le(lenptr)
		require.True(t, ok)
		require.Equal(t, uint32(len("A=1\x00B=2")), n)

		untouched, ok := m.Memory().Read(bufptr, 2)
		require.True(t, ok)
		require.Equal(t, []byte{0, 0}, untouched)
	})

	t.Run("a missing file writes nothing", func(t *testing.T) {
		m := module(t)

		errcode := ffiegenv.Environ(filepath.Join(t.TempDir(), "missing.env"))(t.Context(), m, math.MaxInt64, bufptr, 1024, lenptr)
		require.Zero(t, errcode)

		n, _ := read(t, m)
		require.Zero(t, n)
	})
}
