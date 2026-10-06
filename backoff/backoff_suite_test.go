package backoff_test

import (
	"os"

	"github.com/egdaemon/eg/internal/testx"

	"testing"
)

func TestMain(m *testing.M) {
	testx.Logging()
	os.Exit(m.Run())
}
