package notary

import (
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/sshx"
	"github.com/james-lawrence/eg/internal/testingx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("newAutoSignerPath", func() {
	It("should succeed when no key exists", func() {
		_, err := newAutoSignerPath(
			filepath.Join(testingx.TempDir(), DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		Expect(err).To(Succeed())
	})

	It("should fail when unable to write to disk", func() {
		tmp := testingx.TempDir()
		os.Chmod(tmp, 0400)
		_, err := newAutoSignerPath(
			filepath.Join(tmp, DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		Expect(err).ToNot(Succeed())
	})
})
