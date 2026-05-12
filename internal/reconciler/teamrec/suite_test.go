package teamrec

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTeamRec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TeamRec Suite")
}
