package notary

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/userx"
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
		if userx.CurrentUserOrDefault(userx.Root()).Uid == "0" {
			log.Println("ROOT USER")
			return
		}
		tmp := testx.TempDir()
		os.Stat(tmp)
		Expect(os.Chmod(tmp, 0100)).To(Succeed())
		_, err := newAutoSignerPath(
			filepath.Join(tmp, DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		Expect(err).ToNot(Succeed())
	})
})
