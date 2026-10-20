package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v93/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileAutolinks with inline autolinks", func() {
	var (
		ctx              context.Context
		mockClient       *ghclientmock.MockGitHubClientWrapper
		repo             *v1alpha1.Repository
		preset           *v1alpha1.AutolinksPreset
		err              error
		listCalled       bool
		currentAutolinks []*github.Autolink
		created          []github.CreateAutolinkRequest
		deletedIDs       []int64
	)

	autolink := func(prefix, url string) v1alpha1.Autolink {
		return v1alpha1.Autolink{KeyPrefix: prefix, URLTemplate: url, IsAlphanumeric: true}
	}
	ghAutolink := func(id int64, prefix, url string) *github.Autolink {
		return &github.Autolink{ID: new(id), KeyPrefix: new(prefix), URLTemplate: new(url), IsAlphanumeric: new(true)}
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		repo = &v1alpha1.Repository{
			Name: "test-repo", Namespace: "default",
			Spec: v1alpha1.RepositorySpec{Name: "test-repo"},
		}
		preset = &v1alpha1.AutolinksPreset{
			Name: "preset-1", Namespace: "default",
			Spec: v1alpha1.AutolinksPresetSpec{AutolinkList: []v1alpha1.Autolink{autolink("JIRA-", "https://jira/<num>")}},
		}
		listCalled = false
		currentAutolinks = nil
		created = nil
		deletedIDs = nil
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(repo, preset).Build()

		mockClient.ListAllAutolinksFunc = func(_ context.Context, _, _ string) ([]*github.Autolink, error) {
			listCalled = true
			return currentAutolinks, nil
		}
		mockClient.CreateAutolinkFunc = func(_ context.Context, _, _ string, a github.CreateAutolinkRequest) error {
			created = append(created, a)
			return nil
		}
		mockClient.DeleteAutolinkFunc = func(_ context.Context, _, _ string, id int64) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		}

		rec := &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client:   mockClient,
				Resource: GitHubRepoIdentifier{Owner: "test-org", Name: "test-repo"},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{Client: k8sClient, Resource: repo},
		}
		err = rec.reconcileAutolinks(ctx)
	})

	Context("when an inline autolink does not exist on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Autolinks = []v1alpha1.Autolink{autolink("GH-", "https://gh/<num>")}
		})

		It("creates it", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(HaveLen(1))
			Expect(created[0].KeyPrefix).To(Equal("GH-"))
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when an inline autolink already exists on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Autolinks = []v1alpha1.Autolink{autolink("GH-", "https://gh/<num>")}
			currentAutolinks = []*github.Autolink{ghAutolink(1, "GH-", "https://gh/<num>")}
		})

		It("does nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when an inline autolink was removed from the spec", func() {
		BeforeEach(func() {
			currentAutolinks = []*github.Autolink{ghAutolink(1, "GH-", "https://gh/<num>")}
		})

		It("deletes the orphaned autolink", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedIDs).To(ConsistOf(int64(1)))
		})
	})

	Context("when inline autolinks and presets are combined", func() {
		BeforeEach(func() {
			repo.Spec.AutolinksPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Autolinks = []v1alpha1.Autolink{autolink("GH-", "https://gh/<num>")}
		})

		It("creates the union", func() {
			Expect(err).NotTo(HaveOccurred())
			prefixes := []string{}
			for _, a := range created {
				prefixes = append(prefixes, a.KeyPrefix)
			}
			Expect(prefixes).To(ConsistOf("JIRA-", "GH-"))
		})
	})

	Context("when an inline autolink is identical to a preset autolink", func() {
		BeforeEach(func() {
			repo.Spec.AutolinksPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Autolinks = []v1alpha1.Autolink{autolink("JIRA-", "https://jira/<num>")}
		})

		It("creates it once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(HaveLen(1))
		})
	})

	Context("when an inline autolink conflicts with a preset autolink", func() {
		BeforeEach(func() {
			repo.Spec.AutolinksPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Autolinks = []v1alpha1.Autolink{autolink("JIRA-", "https://other/<num>")}
			currentAutolinks = []*github.Autolink{ghAutolink(1, "OLD-", "https://old/<num>")}
		})

		It("returns a conflict error and makes no GitHub calls", func() {
			var conflictErr *AutolinkConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Sources).To(ConsistOf("autolinksPreset/preset-1[0]", "inline[0]"))
			Expect(listCalled).To(BeFalse())
			Expect(created).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})
})
