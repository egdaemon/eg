package rsax

import (
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
)

// these tests are just to test the functionality, not the actual randomness.
var _ = Describe("NewSHA512CSPRNG", func() {
	DescribeTable("generate data", func(seed string, bits int, expected string) {
		rand := NewSHA512CSPRNG([]byte(seed))
		b := make([]byte, bits)
		n, err := rand.Read(b)
		Expect(err).To(Succeed())
		Expect(n).To(Equal(bits))
		// fmt.Println("output", bits, hex.EncodeToString(b))
		Expect(hex.EncodeToString(b)).To(Equal(expected))
	},
		Entry("example 1",
			"helloworld",
			128,
			"232aed2d2c04bf3b8a925de4eb843d709b036eee500f4b9a1a65b7a5113f087cf5f9dbc67ace94a565aa03ba6c912acdcb3c9b0cfbd9e0cf3dfcc2e1c402ead83febe32d39b210f92b160756bf8cb0f0f3f93e2c43fd85359b4616ad4720ad07bedcd9e9071722710b95e736c2d97048f02083ef9892b661e0b583e2212f1cda",
		),
		Entry("example 2",
			"helloworld",
			256,
			"232aed2d2c04bf3b8a925de4eb843d709b036eee500f4b9a1a65b7a5113f087cf5f9dbc67ace94a565aa03ba6c912acdcb3c9b0cfbd9e0cf3dfcc2e1c402ead83febe32d39b210f92b160756bf8cb0f0f3f93e2c43fd85359b4616ad4720ad07bedcd9e9071722710b95e736c2d97048f02083ef9892b661e0b583e2212f1cdaeaa0b93f2d2d70f3fb9ce60414f847ff6c3a0e022c89ddfaf8e0e764f76ea2da1774fc2fccc28e33a32166be2ca83569111e786a1cccb720a9fe03d3019e6b2162bcc94d74ea51cd33d44147e2c179f6eaef947253833cb5cef7a80fd117de5d51bf2440fa5ff91bb300477c6212d3d9bda969b615cdf60aad6a77d0b99f9344",
		),
		Entry("example 3",
			"helloworld",
			512,
			"232aed2d2c04bf3b8a925de4eb843d709b036eee500f4b9a1a65b7a5113f087cf5f9dbc67ace94a565aa03ba6c912acdcb3c9b0cfbd9e0cf3dfcc2e1c402ead83febe32d39b210f92b160756bf8cb0f0f3f93e2c43fd85359b4616ad4720ad07bedcd9e9071722710b95e736c2d97048f02083ef9892b661e0b583e2212f1cdaeaa0b93f2d2d70f3fb9ce60414f847ff6c3a0e022c89ddfaf8e0e764f76ea2da1774fc2fccc28e33a32166be2ca83569111e786a1cccb720a9fe03d3019e6b2162bcc94d74ea51cd33d44147e2c179f6eaef947253833cb5cef7a80fd117de5d51bf2440fa5ff91bb300477c6212d3d9bda969b615cdf60aad6a77d0b99f93448f119df1a23d2b7368ff9cc47ee39048f73d1e9e7c81686951a36616243ebcd6c17760cbaf0900744da6dd3406637f9d8f600268eed2db34535c498c92b6e00482cca18d13858e075a7634c71f5340a7bcaf04b2e0318775dcd949f72a2f5eb9a9f73a355902c75a7410371f20eb072f293f11a1b75ecd0f07071cdd7c858bfbf0f61684e92814c0c4649374be81841a0ffab6fe19335182a307caa7fdccaa214a9d6b72e9b7c0c84780339c28e16c62cebb6341c70c7d28f5e029e9f4d4dc714a5be2b9219235d8f30437dbcd6b93aa9cb95d45b9633716d3a1388b5bb4452cb5687e783f407988722c2090f4b69568a96c572c8b4e316b194c862902461d9d",
		),
		Entry("example 4",
			"testing",
			128,
			"bfb6b0c2765f4424732e672e48eabdb02e6b6c6ceded5d634016cde63a284795bd7e063f71324d5e0cd6722578fd66d24845928d803837229b2889b6dd9b04f91eb319b8efcfde049866435a469fad4a5b32fd5a12e1c65bcdb0d8897fd0c68d97f0e84c33294fa8ddfba46fcaeba3edffabfccfa010433ce175d9cc980cb822",
		),
	)
})
