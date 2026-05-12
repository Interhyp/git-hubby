package reconciler

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOrgRec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OrgRec Suite")
}
