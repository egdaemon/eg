package egunsafe_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestModuleRuntimeDirectory(t *testing.T) {
	require.Equal(t, "m.099a8057.d", egunsafe.ModuleRuntimeDirectory(uuid.Must(uuid.FromString("01890a5d-ac96-774b-bcce-b302099a8057"))))
}
