package egdmg_test

import (
	"context"
	"log"
	"os"
	"syscall"
	"testing"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/testx"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func TestMain(m *testing.M) {
	testx.Logging()
	go debugx.DumpOnSignal(context.Background(), syscall.SIGUSR2)
	os.Exit(m.Run())
}
