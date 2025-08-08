package egdmg_test

import (
	"context"
	"os"
	"syscall"
	"testing"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/testx"
)

func TestMain(m *testing.M) {
	testx.Logging()
	go debugx.DumpOnSignal(context.Background(), syscall.SIGUSR2)
	os.Exit(m.Run())
}
