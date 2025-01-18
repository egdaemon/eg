package shell_test

import (
	"os"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
)

func TestMain(m *testing.M) {
	testx.Logging()
	os.Exit(m.Run())
}
