package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// These tests run against the envtest API server (see suite_test.go).
var _ = Describe("CodeSecurityConfiguration CRD validation", func() {
	const namespace = "default"

	newCsc := func(name string, advancedSecurity, codeSecurity, secretProtection *string) *githubv1alpha1.CodeSecurityConfiguration {
		return &githubv1alpha1.CodeSecurityConfiguration{
			Name: name, Namespace: namespace,
			Spec: githubv1alpha1.CodeSecurityConfigurationSpec{
				Name:             name,
				Description:      "test",
				AdvancedSecurity: advancedSecurity,
				CodeSecurity:     codeSecurity,
				SecretProtection: secretProtection,
			},
		}
	}

	DescribeTable("accepts advancedSecurity or the individual fields",
		func(name string, advancedSecurity, codeSecurity, secretProtection *string) {
			csc := newCsc(name, advancedSecurity, codeSecurity, secretProtection)
			Expect(k8sClient.Create(ctx, csc)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, csc)).To(Succeed()) })
		},
		Entry("advancedSecurity only", "csc-advanced-security", new("enabled"), nil, nil),
		Entry("individual fields only", "csc-individual", nil, new("enabled"), new("disabled")),
		Entry("none", "csc-none", nil, nil, nil),
	)

	DescribeTable("rejects advancedSecurity together with the individual fields",
		func(name string, codeSecurity, secretProtection *string) {
			err := k8sClient.Create(ctx, newCsc(name, new("enabled"), codeSecurity, secretProtection))
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), fmt.Sprintf("expected invalid error, got: %v", err))
			Expect(err.Error()).To(ContainSubstring("advancedSecurity must not be set together with codeSecurity or secretProtection"))
		},
		Entry("with codeSecurity", "csc-with-code-security", new("enabled"), nil),
		Entry("with secretProtection", "csc-with-secret-protection", nil, new("disabled")),
	)
})
