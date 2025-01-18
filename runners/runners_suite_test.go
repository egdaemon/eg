package runners_test

import (
	"os"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRunners(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runners Suite")
}

func TestMain(m *testing.M) {
	testx.Logging()
	os.Exit(m.Run())
}
