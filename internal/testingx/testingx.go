package testingx

import "github.com/onsi/ginkgo/v2"

func TempDir() string {
	return ginkgo.GinkgoT().TempDir()
}
