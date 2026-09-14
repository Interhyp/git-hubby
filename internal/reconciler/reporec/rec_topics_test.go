package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ReconcileTopics", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository

		topics        []v1alpha1.Topic
		currentTopics []string

		err                   error
		appliedTopics         []string
		updateTopicsCalled    bool
		getCurrentTopicsError error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default topics (empty)
		topics = []v1alpha1.Topic{}

		// Reset flags and errors
		appliedTopics = []string{}
		updateTopicsCalled = false
		getCurrentTopicsError = nil

		// Set up default mock functions
		mockClient.GetAllTopicsFunc = func(ctx context.Context, owner, repo string) ([]string, error) {
			return currentTopics, getCurrentTopicsError
		}

		mockClient.ReplaceAllTopicsFunc = func(ctx context.Context, owner, repo string, topics []string) error {
			updateTopicsCalled = true
			appliedTopics = topics
			return nil
		}
	})

	JustBeforeEach(func() {
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name: "test-repo",
				About: v1alpha1.About{
					Topics: topics,
				},
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(repo).
			WithStatusSubresource(repo).
			Build()

		rec = &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client: mockClient,
				Resource: GitHubRepoIdentifier{
					Owner: "test-org",
					Name:  "test-repo",
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}

		err = rec.reconcileTopics(ctx)

	})

	Context("when no topics are set in spec", func() {
		BeforeEach(func() {
			topics = []v1alpha1.Topic{}
			// Current topics also include defaults for required topics
			currentTopics = []string{}
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			// No update because current already matches desired (defaults)
			Expect(updateTopicsCalled).To(BeFalse())
		})
	})

	Context("when topics match current topics", func() {
		BeforeEach(func() {
			topics = []v1alpha1.Topic{
				{
					Name: "foo",
				}, {
					Name: "bar",
				},
			}
			currentTopics = []string{"foo", "bar"}
		})

		It("should skip update when topics match", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateTopicsCalled).To(BeFalse())
		})
	})

	Context("when creating new topics", func() {
		BeforeEach(func() {
			topics = []v1alpha1.Topic{
				{
					Name: "foo",
				}, {
					Name: "bar",
				},
			}
			currentTopics = []string{}
		})

		It("should create new topics successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateTopicsCalled).To(BeTrue())
			// Mapper returns all 2 topics
			Expect(appliedTopics).To(HaveLen(2))
			Expect(appliedTopics).To(ContainElements("foo", "bar"))
		})
	})

	Context("when updating existing topics", func() {
		BeforeEach(func() {
			topics = []v1alpha1.Topic{
				{
					Name: "fooz",
				}, {
					Name: "barz",
				},
			}
			currentTopics = []string{"foo", "baz"}
		})

		It("should update topics successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateTopicsCalled).To(BeTrue())
			Expect(appliedTopics).To(HaveLen(2))
			Expect(appliedTopics).To(ContainElements("fooz", "barz"))
		})
	})

	Context("when removing topics", func() {
		BeforeEach(func() {
			currentTopics = []string{"foo", "baz"}
		})

		It("should remove topics successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateTopicsCalled).To(BeTrue())
			Expect(appliedTopics).To(BeEmpty())
		})
	})

	Context("when GetAllTopics returns an error", func() {
		BeforeEach(func() {
			getCurrentTopicsError = errors.New("get topics failed")
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("get topics failed"))
			Expect(updateTopicsCalled).To(BeFalse())
		})
	})

	Context("when ReplaceAllTopics returns an error", func() {
		BeforeEach(func() {
			mockClient.ReplaceAllTopicsFunc = func(ctx context.Context, owner, repo string, topics []string) error {
				updateTopicsCalled = true
				return errors.New("replace topics failed")
			}
			topics = []v1alpha1.Topic{
				{
					Name: "foo",
				},
			}
			currentTopics = []string{}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("replace topics failed"))
			Expect(updateTopicsCalled).To(BeTrue())
		})
	})

	Context("test helper functions", func() {
		It("should kebab-case, deduplicate and sort", func() {
			in := []string{"FooBar", "foo-bar", "baz", "Baz"}
			out := uniqueKebabCasedSorted(in)
			Expect(out).To(Equal([]string{"baz", "foo-bar"}))
		})

		It("should handle empty input", func() {
			Expect(uniqueKebabCasedSorted([]string{})).To(BeEmpty())
		})
	})
})
