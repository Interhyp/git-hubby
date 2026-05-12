package spreading

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSpreading(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Spreading Suite")
}
