package rulesets

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRulesets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rulesets Suite")
}
