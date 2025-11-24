package hash

import (
	"encoding/hex"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestHash(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Package: github.com/stellar/go-stellar-sdk/hash")
}

type HashCase struct {
	Msg  string
	Hash string
}

var _ = DescribeTable("Hash()",
	func(c HashCase) {
		sig := Hash([]byte(c.Msg))
		actual := hex.EncodeToString(sig[:])

		Expect(actual).To(Equal(c.Hash))
	},

	Entry("hello world", HashCase{
		"hello world",
		"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
	}),
	Entry("this is a message", HashCase{
		"this is a message",
		"cee86e2a6c441f1e308d16a3db20a8fa8fae2a45730b48ca2c0c61e159af7e78",
	}),
)
