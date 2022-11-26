package rsax

import (
	"crypto/md5"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
)

// these tests are just to test the functionality, not the actual randomness.
var _ = Describe("AutoDeterministic", func() {
	DescribeTable("generate data", func(seed string, bits int, expected string) {
		pkey, err := Deterministic([]byte(seed), bits)
		Expect(err).To(Succeed())
		digest := md5.Sum(pkey)
		Expect(hex.EncodeToString(digest[:])).To(Equal(expected))
	},
		Entry("example 1",
			"helloworld",
			4096,
			"88b3d0f71f96aedc008771cdb2706626",
		),
	)
})
