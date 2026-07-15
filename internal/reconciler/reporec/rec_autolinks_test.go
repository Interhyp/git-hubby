package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ReconcileAutolinks", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository
		preset     *v1alpha1.AutolinksPreset

		autolinks        []v1alpha1.Autolink
		currentAutolinks []*github.Autolink

		err                       error
		appliedAutolinks          []*github.AutolinkOptions
		createAutolinkCalled      bool
		deletedAutolinkIDs        []int64
		deleteAutolinkCalled      bool
		getCurrentAutolionksError error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default autolinks (empty)
		autolinks = []v1alpha1.Autolink{}

		// Reset flags and errors
		appliedAutolinks = []*github.AutolinkOptions{}
		createAutolinkCalled = false
		deletedAutolinkIDs = []int64{}
		deleteAutolinkCalled = false
		getCurrentAutolionksError = nil

		// Set up default mock functions
		mockClient.ListAllAutolinksFunc = func(ctx context.Context, owner, repo string) ([]*github.Autolink, error) {
			return currentAutolinks, getCurrentAutolionksError
		}

		mockClient.CreateAutolinkFunc = func(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error {
			createAutolinkCalled = true
			appliedAutolinks = append(appliedAutolinks, autolink)
			return nil
		}

		mockClient.DeleteAutolinkFunc = func(ctx context.Context, owner, repo string, autolinkID int64) error {
			deleteAutolinkCalled = true
			deletedAutolinkIDs = append(deletedAutolinkIDs, autolinkID)
			return nil
		}
	})

	JustBeforeEach(func() {
		preset = &v1alpha1.AutolinksPreset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-autolinks-preset",
				Namespace: "default",
			},
			Spec: v1alpha1.AutolinksPresetSpec{
				AutolinkList: autolinks,
			},
		}

		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name: "test-repo",
				AutolinksPresetList: []v1.LocalObjectReference{
					{Name: "test-autolinks-preset"},
				},
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(preset, repo).
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

		err = rec.reconcileAutolinks(ctx)

	})

	Context("when no autolinks are set in spec", func() {
		BeforeEach(func() {
			autolinks = []v1alpha1.Autolink{}
			// Current topics also include defaults for required topics
			currentAutolinks = []*github.Autolink{}
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			// No update because current already matches desired (defaults)
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeFalse())
		})
	})

	Context("when autolinks match current topics", func() {
		BeforeEach(func() {
			autolinks = []v1alpha1.Autolink{
				{
					KeyPrefix:      "foo",
					URLTemplate:    "https://example.com/foo",
					IsAlphanumeric: false,
				}, {
					KeyPrefix:      "bar",
					URLTemplate:    "https://example.com/bar",
					IsAlphanumeric: false,
				},
			}
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("foo"),
					URLTemplate:    new("https://example.com/foo"),
					IsAlphanumeric: new(false),
				}, {
					ID:             new(int64(23456)),
					KeyPrefix:      new("bar"),
					URLTemplate:    new("https://example.com/bar"),
					IsAlphanumeric: new(false),
				},
			}
		})

		It("should skip update when autolinks match", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeFalse())
		})
	})

	Context("when creating new autolinks", func() {
		BeforeEach(func() {
			autolinks = []v1alpha1.Autolink{
				{
					KeyPrefix:      "foo",
					URLTemplate:    "https://example.com/foo",
					IsAlphanumeric: false,
				}, {
					KeyPrefix:      "bar",
					URLTemplate:    "https://example.com/bar",
					IsAlphanumeric: false,
				},
			}
			currentAutolinks = []*github.Autolink{}
		})

		It("should create new autolinks successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			// Mapper returns all 2 autolinks
			Expect(appliedAutolinks).To(HaveLen(2))

			// Check that both autolinks exist (order is non-deterministic due to map iteration)
			keyPrefixes := make(map[string]*github.AutolinkOptions)
			for _, autolink := range appliedAutolinks {
				keyPrefixes[*autolink.KeyPrefix] = autolink
			}

			Expect(keyPrefixes).To(HaveKey("foo"))
			Expect(*keyPrefixes["foo"].KeyPrefix).To(Equal("foo"))
			Expect(*keyPrefixes["foo"].URLTemplate).To(Equal("https://example.com/foo"))
			Expect(*keyPrefixes["foo"].IsAlphanumeric).To(BeFalse())

			Expect(keyPrefixes).To(HaveKey("bar"))
			Expect(*keyPrefixes["bar"].KeyPrefix).To(Equal("bar"))
			Expect(*keyPrefixes["bar"].URLTemplate).To(Equal("https://example.com/bar"))
			Expect(*keyPrefixes["bar"].IsAlphanumeric).To(BeFalse())
		})
	})

	Context("when updating existing autolinks", func() {
		BeforeEach(func() {
			autolinks = []v1alpha1.Autolink{
				{
					KeyPrefix:      "foo",
					URLTemplate:    "https://example.com/foo",
					IsAlphanumeric: false,
				}, {
					KeyPrefix:      "bar",
					URLTemplate:    "https://example.com/bar",
					IsAlphanumeric: false,
				},
			}
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("fooz"),
					URLTemplate:    new("https://example.com/fooz"),
					IsAlphanumeric: new(false),
				}, {
					ID:             new(int64(23456)),
					KeyPrefix:      new("bar"),
					URLTemplate:    new("https://example.com/bar"),
					IsAlphanumeric: new(false),
				},
			}
		})

		It("should update autolinks successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(deleteAutolinkCalled).To(BeTrue())
			// Mapper returns all 1 autolinks
			Expect(appliedAutolinks).To(HaveLen(1))
			Expect(*appliedAutolinks[0].KeyPrefix).To(Equal("foo"))
			Expect(*appliedAutolinks[0].URLTemplate).To(Equal("https://example.com/foo"))
			Expect(*appliedAutolinks[0].IsAlphanumeric).To(BeFalse())
			Expect(deletedAutolinkIDs).To(HaveLen(1))
			Expect(deletedAutolinkIDs).To(ContainElements(int64(12345)))
		})
	})

	Context("when removing autolinks", func() {
		BeforeEach(func() {
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("fooz"),
					URLTemplate:    new("https://example.com/fooz"),
					IsAlphanumeric: new(false),
				}, {
					ID:             new(int64(23456)),
					KeyPrefix:      new("bar"),
					URLTemplate:    new("https://example.com/bar"),
					IsAlphanumeric: new(false),
				},
			}
		})

		It("should remove autolinks successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeTrue())
			Expect(deletedAutolinkIDs).To(HaveLen(2))
			Expect(deletedAutolinkIDs).To(ContainElements(int64(12345), int64(23456)))
		})
	})

	Context("when ListAllAutolinks returns an error", func() {
		BeforeEach(func() {
			getCurrentAutolionksError = errors.New("get autolinks failed")
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("get autolinks failed"))
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeFalse())
		})
	})

	Context("when CreateAutolink returns an error", func() {
		BeforeEach(func() {
			mockClient.CreateAutolinkFunc = func(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error {
				createAutolinkCalled = true
				return errors.New("add autolink failed")
			}
			autolinks = []v1alpha1.Autolink{
				{
					KeyPrefix:      "foo",
					URLTemplate:    "https://example.com/foo",
					IsAlphanumeric: false,
				}, {
					KeyPrefix:      "bar",
					URLTemplate:    "https://example.com/bar",
					IsAlphanumeric: false,
				},
			}
			currentAutolinks = []*github.Autolink{}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("add autolink failed"))
			Expect(createAutolinkCalled).To(BeTrue())
		})
	})

	Context("when DeleteAutolink returns an error", func() {
		BeforeEach(func() {
			mockClient.DeleteAutolinkFunc = func(ctx context.Context, owner, repo string, id int64) error {
				deleteAutolinkCalled = true
				return errors.New("delete autolink failed")
			}
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("fooz"),
					URLTemplate:    new("https://example.com/fooz"),
					IsAlphanumeric: new(false),
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("delete autolink failed"))
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeTrue())
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

var _ = Describe("ReconcileAutolinks with Multiple Presets", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository
		preset1    *v1alpha1.AutolinksPreset
		preset2    *v1alpha1.AutolinksPreset
		preset3    *v1alpha1.AutolinksPreset

		currentAutolinks []*github.Autolink

		err                       error
		appliedAutolinks          []*github.AutolinkOptions
		createAutolinkCalled      bool
		deletedAutolinkIDs        []int64
		deleteAutolinkCalled      bool
		getCurrentAutolionksError error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Reset flags and errors
		appliedAutolinks = []*github.AutolinkOptions{}
		createAutolinkCalled = false
		deletedAutolinkIDs = []int64{}
		deleteAutolinkCalled = false
		getCurrentAutolionksError = nil
		currentAutolinks = []*github.Autolink{}

		// Set up default mock functions
		mockClient.ListAllAutolinksFunc = func(ctx context.Context, owner, repo string) ([]*github.Autolink, error) {
			return currentAutolinks, getCurrentAutolionksError
		}

		mockClient.CreateAutolinkFunc = func(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error {
			createAutolinkCalled = true
			appliedAutolinks = append(appliedAutolinks, autolink)
			return nil
		}

		mockClient.DeleteAutolinkFunc = func(ctx context.Context, owner, repo string, autolinkID int64) error {
			deleteAutolinkCalled = true
			deletedAutolinkIDs = append(deletedAutolinkIDs, autolinkID)
			return nil
		}
	})

	Context("when merging two presets with non-overlapping autolinks", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-jira",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://jira.example.com/browse/TICKET-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-github",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "GH-",
							URLTemplate:    "https://github.com/org/repo/issues/<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "PR-",
							URLTemplate:    "https://github.com/org/repo/pull/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-jira"},
						{Name: "preset-github"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should create all autolinks from both presets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(deleteAutolinkCalled).To(BeFalse())
			Expect(appliedAutolinks).To(HaveLen(4))

			// Verify all four autolinks are present
			keyPrefixes := make([]string, 0, 4)
			for _, autolink := range appliedAutolinks {
				keyPrefixes = append(keyPrefixes, *autolink.KeyPrefix)
			}
			Expect(keyPrefixes).To(ConsistOf("JIRA-", "TICKET-", "GH-", "PR-"))
		})
	})

	Context("when merging two presets with duplicate autolinks", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-base",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://jira.example.com/browse/TICKET-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-duplicate",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "GH-",
							URLTemplate:    "https://github.com/org/repo/issues/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-base"},
						{Name: "preset-duplicate"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should deduplicate and create only unique autolinks", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(deleteAutolinkCalled).To(BeFalse())
			Expect(appliedAutolinks).To(HaveLen(3))

			// Verify only unique autolinks are present (JIRA- should appear only once)
			keyPrefixes := make([]string, 0, 3)
			for _, autolink := range appliedAutolinks {
				keyPrefixes = append(keyPrefixes, *autolink.KeyPrefix)
			}
			Expect(keyPrefixes).To(ConsistOf("JIRA-", "TICKET-", "GH-"))
		})
	})

	Context("when merging three presets with partial overlaps", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-jira",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://jira.example.com/browse/TICKET-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-github",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "GH-",
							URLTemplate:    "https://github.com/org/repo/issues/<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://jira.example.com/browse/TICKET-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset3 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-custom",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "CUSTOM-",
							URLTemplate:    "https://custom.example.com/<num>",
							IsAlphanumeric: false,
						},
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-jira"},
						{Name: "preset-github"},
						{Name: "preset-custom"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, preset3, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should deduplicate and create only unique autolinks from all three presets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(deleteAutolinkCalled).To(BeFalse())
			Expect(appliedAutolinks).To(HaveLen(4))

			// Verify only unique autolinks are present (JIRA- and TICKET- should appear only once each)
			keyPrefixes := make([]string, 0, 4)
			for _, autolink := range appliedAutolinks {
				keyPrefixes = append(keyPrefixes, *autolink.KeyPrefix)
			}
			Expect(keyPrefixes).To(ConsistOf("JIRA-", "TICKET-", "GH-", "CUSTOM-"))
		})
	})

	Context("when merging presets and some autolinks already exist in GitHub", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-jira",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://jira.example.com/browse/TICKET-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-github",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "GH-",
							URLTemplate:    "https://github.com/org/repo/issues/<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "PR-",
							URLTemplate:    "https://github.com/org/repo/pull/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			// JIRA- and GH- already exist in GitHub
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("JIRA-"),
					URLTemplate:    new("https://jira.example.com/browse/JIRA-<num>"),
					IsAlphanumeric: new(true),
				},
				{
					ID:             new(int64(23456)),
					KeyPrefix:      new("GH-"),
					URLTemplate:    new("https://github.com/org/repo/issues/<num>"),
					IsAlphanumeric: new(true),
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-jira"},
						{Name: "preset-github"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should only create missing autolinks and preserve existing ones", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(deleteAutolinkCalled).To(BeFalse())
			Expect(appliedAutolinks).To(HaveLen(2))

			// Verify only missing autolinks are created (TICKET- and PR-)
			keyPrefixes := make([]string, 0, 2)
			for _, autolink := range appliedAutolinks {
				keyPrefixes = append(keyPrefixes, *autolink.KeyPrefix)
			}
			Expect(keyPrefixes).To(ConsistOf("TICKET-", "PR-"))
		})
	})

	Context("when merging presets with GitHub having extra autolinks not in presets", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-jira",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-github",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "GH-",
							URLTemplate:    "https://github.com/org/repo/issues/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			// GitHub has extra autolinks (OLD- and LEGACY-) that are not in presets
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("JIRA-"),
					URLTemplate:    new("https://jira.example.com/browse/JIRA-<num>"),
					IsAlphanumeric: new(true),
				},
				{
					ID:             new(int64(23456)),
					KeyPrefix:      new("GH-"),
					URLTemplate:    new("https://github.com/org/repo/issues/<num>"),
					IsAlphanumeric: new(true),
				},
				{
					ID:             new(int64(34567)),
					KeyPrefix:      new("OLD-"),
					URLTemplate:    new("https://old.example.com/<num>"),
					IsAlphanumeric: new(true),
				},
				{
					ID:             new(int64(45678)),
					KeyPrefix:      new("LEGACY-"),
					URLTemplate:    new("https://legacy.example.com/<num>"),
					IsAlphanumeric: new(false),
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-jira"},
						{Name: "preset-github"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should remove autolinks not defined in presets and preserve those that are", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeTrue())
			Expect(deletedAutolinkIDs).To(HaveLen(2))
			Expect(deletedAutolinkIDs).To(ConsistOf(int64(34567), int64(45678)))
		})
	})

	Context("when merging presets with full reconciliation (add, remove, keep)", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-jira",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://jira.example.com/browse/TICKET-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-github",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "GH-",
							URLTemplate:    "https://github.com/org/repo/issues/<num>",
							IsAlphanumeric: true,
						},
						{
							KeyPrefix:      "PR-",
							URLTemplate:    "https://github.com/org/repo/pull/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			// GitHub has: JIRA- (keep), OLD- (remove), and missing: TICKET-, GH-, PR-
			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("JIRA-"),
					URLTemplate:    new("https://jira.example.com/browse/JIRA-<num>"),
					IsAlphanumeric: new(true),
				},
				{
					ID:             new(int64(99999)),
					KeyPrefix:      new("OLD-"),
					URLTemplate:    new("https://old.example.com/<num>"),
					IsAlphanumeric: new(true),
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-jira"},
						{Name: "preset-github"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should add missing autolinks, remove orphaned ones, and keep existing desired ones", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(deleteAutolinkCalled).To(BeTrue())

			// Should create 3 missing autolinks (TICKET-, GH-, PR-)
			Expect(appliedAutolinks).To(HaveLen(3))
			keyPrefixes := make([]string, 0, 3)
			for _, autolink := range appliedAutolinks {
				keyPrefixes = append(keyPrefixes, *autolink.KeyPrefix)
			}
			Expect(keyPrefixes).To(ConsistOf("TICKET-", "GH-", "PR-"))

			// Should delete 1 orphaned autolink (OLD-)
			Expect(deletedAutolinkIDs).To(HaveLen(1))
			Expect(deletedAutolinkIDs).To(ContainElement(int64(99999)))
		})
	})

	Context("when a preset is missing in the cluster", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-jira",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "JIRA-",
							URLTemplate:    "https://jira.example.com/browse/JIRA-<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			// preset-missing does not exist in the cluster
			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-jira"},
						{Name: "preset-missing"}, // This preset doesn't exist
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should return an error indicating the preset is missing", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("preset-missing"))
		})
	})

	Context("when merging presets with different IsAlphanumeric values for same keyPrefix", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-alpha",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://example.com/ticket/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-nonalpha",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://example.com/ticket/<num>",
							IsAlphanumeric: false, // Different IsAlphanumeric value
						},
					},
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-alpha"},
						{Name: "preset-nonalpha"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should treat them as different autolinks and create both", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(appliedAutolinks).To(HaveLen(2))

			// Both should be created because IsAlphanumeric differs
			alphaCount := 0
			nonAlphaCount := 0
			for _, autolink := range appliedAutolinks {
				Expect(*autolink.KeyPrefix).To(Equal("TICKET-"))
				Expect(*autolink.URLTemplate).To(Equal("https://example.com/ticket/<num>"))
				if *autolink.IsAlphanumeric {
					alphaCount++
				} else {
					nonAlphaCount++
				}
			}
			Expect(alphaCount).To(Equal(1))
			Expect(nonAlphaCount).To(Equal(1))
		})
	})

	Context("when merging empty presets", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-empty1",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-empty2",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{},
				},
			}

			currentAutolinks = []*github.Autolink{
				{
					ID:             new(int64(12345)),
					KeyPrefix:      new("OLD-"),
					URLTemplate:    new("https://old.example.com/<num>"),
					IsAlphanumeric: new(true),
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-empty1"},
						{Name: "preset-empty2"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should remove all existing autolinks when presets are empty", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeFalse())
			Expect(deleteAutolinkCalled).To(BeTrue())
			Expect(deletedAutolinkIDs).To(HaveLen(1))
			Expect(deletedAutolinkIDs).To(ContainElement(int64(12345)))
		})
	})

	Context("when preset order matters for deduplication", func() {
		BeforeEach(func() {
			preset1 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-first",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://first.example.com/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			preset2 = &v1alpha1.AutolinksPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "preset-second",
					Namespace: "default",
				},
				Spec: v1alpha1.AutolinksPresetSpec{
					AutolinkList: []v1alpha1.Autolink{
						{
							KeyPrefix:      "TICKET-",
							URLTemplate:    "https://second.example.com/<num>",
							IsAlphanumeric: true,
						},
					},
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					AutolinksPresetList: []v1.LocalObjectReference{
						{Name: "preset-first"},
						{Name: "preset-second"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(preset1, preset2, repo).
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

			err = rec.reconcileAutolinks(ctx)
		})

		It("should treat different URLs as different autolinks even with same keyPrefix", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createAutolinkCalled).To(BeTrue())
			Expect(appliedAutolinks).To(HaveLen(2))

			// Both should be created because URLs differ
			urls := make([]string, 0, 2)
			for _, autolink := range appliedAutolinks {
				Expect(*autolink.KeyPrefix).To(Equal("TICKET-"))
				urls = append(urls, *autolink.URLTemplate)
			}
			Expect(urls).To(ConsistOf("https://first.example.com/<num>", "https://second.example.com/<num>"))
		})
	})
})
