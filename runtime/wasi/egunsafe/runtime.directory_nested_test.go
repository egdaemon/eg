package egunsafe_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/stretchr/testify/require"
)

func TestModuleRuntimeDirectory(t *testing.T) {
	require.Equal(t, "m.099a8057.d", egunsafe.ModuleRuntimeDirectory(errorsx.Must(uuidx.FromString("01890a5d-ac96-774b-bcce-b302099a8057"))))
}
