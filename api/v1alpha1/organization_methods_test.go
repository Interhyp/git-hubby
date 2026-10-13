package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization.IsActionsDisabled", func() {
	withEnabledRepositories := func(value *string) *Organization {
		return &Organization{Spec: OrganizationSpec{ActionsSettings: ActionsSettings{EnabledRepositories: value}}}
	}

	DescribeTable("evaluates spec.actionsSettings.enabledRepositories",
		func(org *Organization, expected bool) {
			Expect(org.IsActionsDisabled()).To(Equal(expected))
		},
		Entry("nil organization", nil, true),
		Entry("unset", withEnabledRepositories(nil), true),
		Entry(`"none"`, withEnabledRepositories(new("none")), true),
		Entry(`"all"`, withEnabledRepositories(new("all")), false),
		Entry(`"selected"`, withEnabledRepositories(new("selected")), false),
	)
})
