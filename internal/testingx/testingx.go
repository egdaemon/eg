package testingx

import (
	"os"

	"github.com/onsi/gomega"
)

const rootDir = ".tests"

// TempDir generates a tmp directory within the root testing directory for use in tests.
func TempDir() (dir string) {
	var err error
	setup()

	if err = os.MkdirAll(rootDir, 0755); err != nil {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	if dir, err = os.MkdirTemp(rootDir, "tmp"); err != nil {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	return dir
}

func setup() {
	var (
		err error
	)

	if err = os.MkdirAll(rootDir, 0755); err != nil {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}
