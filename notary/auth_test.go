package notary

import (
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/testx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("newAutoSignerPath", func() {
	It("should succeed when no key exists", func() {
		_, err := newAutoSignerPath(
			filepath.Join(testx.TempDir(), DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		Expect(err).To(Succeed())
	})

	It("should fail when unable to write to disk", func() {
		tmp := testx.TempDir()
		Expect(os.Chmod(tmp, 0400)).To(Succeed())
		_, err := newAutoSignerPath(
			filepath.Join(tmp, DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		Expect(err).ToNot(Succeed())
	})
})
