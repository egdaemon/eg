package notary_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNotary(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Notary Suite")
}
