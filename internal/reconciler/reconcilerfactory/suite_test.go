package reconcilerfactory

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReconcilerFactory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ReconcilerFactory Suite")
}
