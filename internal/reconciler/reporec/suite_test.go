package reporec

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRepoRec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RepoRec Suite")
}
