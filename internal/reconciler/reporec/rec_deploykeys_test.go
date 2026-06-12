package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileDeployKeys", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository

		deployKeys        []v1alpha1.DeployKey
		currentDeployKeys []*github.Key

		err                       error
		appliedDeployKeys         []*github.Key
		createDeployKeyCalled     bool
		deletedDeployKeyIDs       []int64
		deleteDeployKeyCalled     bool
		getCurrentDeployKeysError error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default deploy keys (empty)
		deployKeys = []v1alpha1.DeployKey{}

		// Reset flags and errors
		appliedDeployKeys = []*github.Key{}
		createDeployKeyCalled = false
		deletedDeployKeyIDs = []int64{}
		deleteDeployKeyCalled = false
		getCurrentDeployKeysError = nil

		// Set up default mock functions
		mockClient.ListAllDeployKeysFunc = func(ctx context.Context, owner, repo string) ([]*github.Key, error) {
			return currentDeployKeys, getCurrentDeployKeysError
		}

		mockClient.CreateDeployKeyFunc = func(ctx context.Context, owner, repo string, key *github.Key) error {
			createDeployKeyCalled = true
			appliedDeployKeys = append(appliedDeployKeys, key)
			return nil
		}

		mockClient.DeleteDeployKeyFunc = func(ctx context.Context, owner, repo string, key int64) error {
			deleteDeployKeyCalled = true
			deletedDeployKeyIDs = append(deletedDeployKeyIDs, key)
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
				Name:          "test-repo",
				DeployKeyList: deployKeys,
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

		err = rec.reconcileDeployKeys(ctx)

	})

	Context("when no deploy keys are set in spec", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{}
			// Current topics also include defaults for required topics
			currentDeployKeys = []*github.Key{}
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			// No update because current already matches desired (defaults)
			Expect(createDeployKeyCalled).To(BeFalse())
			Expect(deleteDeployKeyCalled).To(BeFalse())
		})
	})

	Context("when deploy key match current topics", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "foo",
					Key:      "random-foo-key",
					ReadOnly: new(false),
				}, {
					Title:    "bar",
					Key:      "random-bar-key",
					ReadOnly: new(false),
				},
			}
			currentDeployKeys = []*github.Key{
				{
					ID:       new(int64(12345)),
					Title:    new("foo"),
					Key:      new("random-foo-key"),
					ReadOnly: new(false),
				}, {
					ID:       new(int64(23456)),
					Title:    new("bar"),
					Key:      new("random-bar-key"),
					ReadOnly: new(false),
				},
			}
		})

		It("should skip update when deploy keys match", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createDeployKeyCalled).To(BeFalse())
			Expect(deleteDeployKeyCalled).To(BeFalse())
		})
	})

	Context("when creating new deploy key", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "foo",
					Key:      "random-foo-key",
					ReadOnly: new(false),
				}, {
					Title:    "bar",
					Key:      "random-bar-key",
					ReadOnly: new(false),
				},
			}
			currentDeployKeys = []*github.Key{}
		})

		It("should create new deploy keys successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createDeployKeyCalled).To(BeTrue())
			// Mapper returns all 2 deploy keys (order doesn't matter due to map iteration)
			Expect(appliedDeployKeys).To(HaveLen(2))
			Expect(appliedDeployKeys).To(ConsistOf(
				And(
					HaveField("Title", Equal(new("foo"))),
					HaveField("Key", Equal(new("random-foo-key"))),
					HaveField("ReadOnly", Equal(new(false))),
				),
				And(
					HaveField("Title", Equal(new("bar"))),
					HaveField("Key", Equal(new("random-bar-key"))),
					HaveField("ReadOnly", Equal(new(false))),
				),
			))
		})
	})

	Context("when updating existing deploy key", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "foo",
					Key:      "random-foo-key",
					ReadOnly: new(false),
				}, {
					Title:    "bar",
					Key:      "random-bar-key",
					ReadOnly: new(false),
				},
			}
			currentDeployKeys = []*github.Key{
				{
					ID:       new(int64(12345)),
					Title:    new("fooz"),
					Key:      new("random-fooz-key"),
					ReadOnly: new(false),
				}, {
					ID:       new(int64(23456)),
					Title:    new("bar"),
					Key:      new("random-bar-key"),
					ReadOnly: new(false),
				},
			}
		})

		It("should update deploy keys successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createDeployKeyCalled).To(BeTrue())
			Expect(deleteDeployKeyCalled).To(BeTrue())
			// Mapper returns all 1 deploy keys
			Expect(appliedDeployKeys).To(HaveLen(1))
			Expect(*appliedDeployKeys[0].Title).To(Equal("foo"))
			Expect(*appliedDeployKeys[0].Key).To(Equal("random-foo-key"))
			Expect(*appliedDeployKeys[0].ReadOnly).To(BeFalse())
			Expect(deletedDeployKeyIDs).To(HaveLen(1))
			Expect(deletedDeployKeyIDs).To(ContainElements(int64(12345)))
		})
	})

	Context("when removing deploy keys", func() {
		BeforeEach(func() {
			currentDeployKeys = []*github.Key{
				{
					ID:       new(int64(12345)),
					Title:    new("fooz"),
					Key:      new("random-fooz-key"),
					ReadOnly: new(false),
				}, {
					ID:       new(int64(23456)),
					Title:    new("bar"),
					Key:      new("random-bar-key"),
					ReadOnly: new(false),
				},
			}
		})

		It("should remove deploy keys successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createDeployKeyCalled).To(BeFalse())
			Expect(deleteDeployKeyCalled).To(BeTrue())
			Expect(deletedDeployKeyIDs).To(HaveLen(2))
			Expect(deletedDeployKeyIDs).To(ContainElements(int64(12345), int64(23456)))
		})
	})

	Context("when ListAllDeployKeys returns an error", func() {
		BeforeEach(func() {
			getCurrentDeployKeysError = errors.New("get deploy keys failed")
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("get deploy keys failed"))
			Expect(createDeployKeyCalled).To(BeFalse())
			Expect(deleteDeployKeyCalled).To(BeFalse())
		})
	})

	Context("when CreateDeployKey returns an error", func() {
		BeforeEach(func() {
			mockClient.CreateDeployKeyFunc = func(ctx context.Context, owner, repo string, key *github.Key) error {
				createDeployKeyCalled = true
				return errors.New("add deploy key failed")
			}
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "foo",
					Key:      "random-foo-key",
					ReadOnly: new(false),
				}, {
					Title:    "bar",
					Key:      "random-bar-key",
					ReadOnly: new(false),
				},
			}
			currentDeployKeys = []*github.Key{}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("add deploy key failed"))
			Expect(createDeployKeyCalled).To(BeTrue())
		})
	})

	Context("when DeleteDeployKey returns an error", func() {
		BeforeEach(func() {
			mockClient.DeleteDeployKeyFunc = func(ctx context.Context, owner, repo string, id int64) error {
				deleteDeployKeyCalled = true
				return errors.New("delete deploy key failed")
			}
			currentDeployKeys = []*github.Key{
				{
					ID:       new(int64(12345)),
					Title:    new("fooz"),
					Key:      new("random-fooz-key"),
					ReadOnly: new(false),
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("delete deploy key failed"))
			Expect(createDeployKeyCalled).To(BeFalse())
			Expect(deleteDeployKeyCalled).To(BeTrue())
		})
	})

	Context("when deploy key has nil ReadOnly field (testing default)", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "readonly-key",
					Key:      "ssh-rsa AAAAB3Nza...",
					ReadOnly: nil, // should default to true
				},
			}
			currentDeployKeys = []*github.Key{}
		})

		It("should create deploy key with true ReadOnly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createDeployKeyCalled).To(BeTrue())
			Expect(appliedDeployKeys).To(HaveLen(1))
			Expect(appliedDeployKeys[0].GetTitle()).To(Equal("readonly-key"))
			// Mapper passes nil through - GitHub will apply its own default (true)
			Expect(appliedDeployKeys[0].ReadOnly).NotTo(BeNil())
			Expect(*appliedDeployKeys[0].ReadOnly).To(BeTrue())
		})
	})

	Context("when deploy key with nil ReadOnly matches GitHub default", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "readonly-key",
					Key:      "ssh-rsa AAAAB3Nza...",
					ReadOnly: nil, // defaults to true
				},
			}
			currentDeployKeys = []*github.Key{
				{
					ID:       new(int64(12345)),
					Title:    new("readonly-key"),
					Key:      new("ssh-rsa AAAAB3Nza..."),
					ReadOnly: new(true), // GitHub's default
				},
			}
		})

		It("should not recreate the deploy key", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createDeployKeyCalled).To(BeFalse())
			Expect(deleteDeployKeyCalled).To(BeFalse())
		})
	})

	Context("when deploy key with nil ReadOnly differs from GitHub", func() {
		BeforeEach(func() {
			deployKeys = []v1alpha1.DeployKey{
				{
					Title:    "readonly-key",
					Key:      "ssh-rsa AAAAB3Nza...",
					ReadOnly: nil, // defaults to true
				},
			}
			currentDeployKeys = []*github.Key{
				{
					ID:       new(int64(12345)),
					Title:    new("readonly-key"),
					Key:      new("ssh-rsa AAAAB3Nza..."),
					ReadOnly: new(false), // differs from default
				},
			}
		})

		It("should recreate the deploy key with correct ReadOnly value", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteDeployKeyCalled).To(BeTrue())
			Expect(createDeployKeyCalled).To(BeTrue())
			Expect(deletedDeployKeyIDs).To(ContainElement(int64(12345)))
			Expect(appliedDeployKeys).To(HaveLen(1))
		})
	})
})
