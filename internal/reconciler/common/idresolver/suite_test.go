package idresolver

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIDResolver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IDResolver Suite")
}
