package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// These tests run against the envtest API server (see suite_test.go). The CRDs being installed successfully
// in BeforeSuite also verifies that the inline presets stay within the CEL cost budget.
var _ = Describe("Inline autolinks, webhooks and webhook ignores CRD validation", func() {
	const namespace = "default"

	newRepo := func(name string, mutate func(*githubv1alpha1.RepositorySpec)) *githubv1alpha1.Repository {
		repo := &githubv1alpha1.Repository{
			Name: name, Namespace: namespace,
			Spec: githubv1alpha1.RepositorySpec{
				Name:            name,
				OrganizationRef: githubv1alpha1.OrganizationRef{Name: "some-org"},
			},
		}
		mutate(&repo.Spec)
		return repo
	}

	autolinks := func(n int, prefix func(int) string) []githubv1alpha1.Autolink {
		result := make([]githubv1alpha1.Autolink, 0, n)
		for i := range n {
			result = append(result, githubv1alpha1.Autolink{KeyPrefix: prefix(i), URLTemplate: "https://example.com/<num>"})
		}
		return result
	}
	webhooks := func(n int) []githubv1alpha1.WebhookSpec {
		result := make([]githubv1alpha1.WebhookSpec, 0, n)
		for i := range n {
			result = append(result, githubv1alpha1.WebhookSpec{PayloadURL: fmt.Sprintf("https://example.com/hook-%d", i)})
		}
		return result
	}
	ignores := func(n int) []githubv1alpha1.WebhookIgnoreSpec {
		result := make([]githubv1alpha1.WebhookIgnoreSpec, 0, n)
		for i := range n {
			result = append(result, githubv1alpha1.WebhookIgnoreSpec{IgnoreURLRegex: new(fmt.Sprintf("^https://ignored-%d/.*", i))})
		}
		return result
	}

	expectInvalid := func(err error, msg string) {
		ExpectWithOffset(1, err).To(HaveOccurred())
		ExpectWithOffset(1, apierrors.IsInvalid(err)).To(BeTrue(), fmt.Sprintf("expected invalid error, got: %v", err))
		ExpectWithOffset(1, err.Error()).To(ContainSubstring(msg))
	}

	It("accepts the maximum number of inline entries", func() {
		repo := newRepo("inline-presets-ok", func(s *githubv1alpha1.RepositorySpec) {
			s.Autolinks = autolinks(25, func(i int) string { return fmt.Sprintf("K%d-", i) })
			s.Webhooks = webhooks(10)
			s.WebhookIgnores = ignores(10)
		})
		Expect(k8sClient.Create(ctx, repo)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, repo)).To(Succeed()) })
	})

	It("rejects duplicate inline autolink keyPrefixes", func() {
		err := k8sClient.Create(ctx, newRepo("inline-autolinks-dup", func(s *githubv1alpha1.RepositorySpec) {
			s.Autolinks = autolinks(2, func(int) string { return "DUP-" })
		}))
		expectInvalid(err, "autolink keyPrefixes must be unique")
	})

	It("rejects more than 25 inline autolinks", func() {
		err := k8sClient.Create(ctx, newRepo("inline-autolinks-too-many", func(s *githubv1alpha1.RepositorySpec) {
			s.Autolinks = autolinks(26, func(i int) string { return fmt.Sprintf("K%d-", i) })
		}))
		expectInvalid(err, "spec.autolinks")
	})

	It("rejects more than 10 inline webhooks", func() {
		err := k8sClient.Create(ctx, newRepo("inline-webhooks-too-many", func(s *githubv1alpha1.RepositorySpec) {
			s.Webhooks = webhooks(11)
		}))
		expectInvalid(err, "spec.webhooks")
	})

	It("rejects more than 10 inline webhook ignores", func() {
		err := k8sClient.Create(ctx, newRepo("inline-ignores-too-many", func(s *githubv1alpha1.RepositorySpec) {
			s.WebhookIgnores = ignores(11)
		}))
		expectInvalid(err, "spec.webhookIgnores")
	})

	It("rejects inline webhook ignores without regex", func() {
		err := k8sClient.Create(ctx, newRepo("inline-ignores-empty", func(s *githubv1alpha1.RepositorySpec) {
			s.WebhookIgnores = []githubv1alpha1.WebhookIgnoreSpec{{}}
		}))
		expectInvalid(err, "ignoreURLRegex must be set for inline webhook ignores")
	})
})
