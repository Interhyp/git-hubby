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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileTeams", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository

		desiredTeams    []v1alpha1.RepositoryTeamPermission
		teamResources   []*v1alpha1.Team
		repositoryTeams []*github.Team

		addCalls    []ghclientmock.TeamCall
		removeCalls []ghclientmock.TeamCall

		listRepoErr error
		addErr      error
		removeErr   error

		err error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		desiredTeams = []v1alpha1.RepositoryTeamPermission{}
		teamResources = []*v1alpha1.Team{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "platform-team", Namespace: "default"},
				Spec: v1alpha1.TeamSpec{
					Name:             "platform-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{{Name: "acme-corp"}},
				},
				Status: v1alpha1.TeamStatus{Slug: new("platform-team")},
			},
		}
		repositoryTeams = []*github.Team{}

		addCalls = []ghclientmock.TeamCall{}
		removeCalls = []ghclientmock.TeamCall{}

		listRepoErr = nil
		addErr = nil
		removeErr = nil

		mockClient.GetAllRepositoryTeamsFunc = func(ctx context.Context, owner, repo string) ([]*github.Team, error) {
			return repositoryTeams, listRepoErr
		}

		mockClient.AddRepositoryTeamFunc = func(ctx context.Context, org, slug, owner, repo, permission string) error {
			addCalls = append(addCalls, ghclientmock.TeamCall{Method: "AddRepositoryTeam", Org: org, Slug: slug, Owner: owner, Repo: repo, Permission: permission})
			return addErr
		}

		mockClient.RemoveTeamFromRepoFunc = func(ctx context.Context, org, slug, owner, repo string) error {
			removeCalls = append(removeCalls, ghclientmock.TeamCall{Method: "RemoveRepositoryTeam", Org: org, Slug: slug, Owner: owner, Repo: repo})
			return removeErr
		}
	})

	JustBeforeEach(func() {
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name:            "test-repo",
				OrganizationRef: v1alpha1.OrganizationRef{Name: "acme-corp"},
				Teams:           desiredTeams,
			},
		}

		objs := []client.Object{repo}
		for _, team := range teamResources {
			objs = append(objs, team)
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
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

		err = rec.reconcileTeams(ctx)
	})

	Context("when desired state matches existing repository team permissions", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
				Permission: "push",
			}}
			repositoryTeams = []*github.Team{{
				Slug:       new("platform-team"),
				Permission: new("push"),
			}}
		})

		It("should not change team assignments", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when repository team is missing", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
				Permission: "maintain",
			}}
		})

		It("should add team permission", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Slug).To(Equal("platform-team"))
			Expect(addCalls[0].Permission).To(Equal("maintain"))
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when permission differs", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
				Permission: "admin",
			}}
			repositoryTeams = []*github.Team{{
				Slug:       new("platform-team"),
				Permission: new("push"),
			}}
		})

		It("should update team permission", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Permission).To(Equal("admin"))
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when team has empty permission in spec", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef: v1alpha1.TeamRef{Name: "platform-team"},
			}}
		})

		It("should default permission to pull", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Permission).To(Equal("pull"))
		})
	})

	Context("when repository has extra team assignments", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
				Permission: "push",
			}}
			repositoryTeams = []*github.Team{
				{Slug: new("platform-team"), Permission: new("push")},
				{Slug: new("security-team"), Permission: new("triage")},
			}
		})

		It("should remove teams not present in spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(HaveLen(1))
			Expect(removeCalls[0].Slug).To(Equal("security-team"))
		})
	})

	Context("when desired team is not found in organization", func() {
		BeforeEach(func() {
			teamResources = []*v1alpha1.Team{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "does-not-exist", Namespace: "default"},
					Spec: v1alpha1.TeamSpec{
						Name:             "does-not-exist",
						OrganizationRefs: []v1alpha1.OrganizationRef{{Name: "some-other-org"}},
					},
					Status: v1alpha1.TeamStatus{Slug: new("does-not-exist")},
				},
			}
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "does-not-exist"},
				Permission: "pull",
			}}
		})

		It("should skip unknown teams and not fail", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when Team CR has not synced its slug to GitHub yet", func() {
		BeforeEach(func() {
			teamResources = []*v1alpha1.Team{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "platform-team", Namespace: "default"},
					Spec: v1alpha1.TeamSpec{
						Name:             "platform-team",
						OrganizationRefs: []v1alpha1.OrganizationRef{{Name: "acme-corp"}},
					},
				},
			}
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
				Permission: "pull",
			}}
		})

		It("should skip the team and not fail", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when referenced Team CRD does not exist", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "missing-team-cr"},
				Permission: "pull",
			}}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when one of several referenced Team CRDs does not exist", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{
				{
					TeamRef:    v1alpha1.TeamRef{Name: "missing-team-cr"},
					Permission: "pull",
				},
				{
					TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
					Permission: "push",
				},
			}
			repositoryTeams = []*github.Team{
				{Slug: new("security-team"), Permission: new("triage")},
			}
		})

		It("should still reconcile the remaining teams and prune stale ones", func() {
			Expect(err).To(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Slug).To(Equal("platform-team"))
			Expect(addCalls[0].Permission).To(Equal("push"))
			Expect(removeCalls).To(HaveLen(1))
			Expect(removeCalls[0].Slug).To(Equal("security-team"))
		})
	})

	Context("when teams is nil (not specified)", func() {
		BeforeEach(func() {
			desiredTeams = nil
			repositoryTeams = []*github.Team{{
				Slug:       new("platform-team"),
				Permission: new("push"),
			}}
		})

		It("should skip reconciliation and leave GitHub untouched", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when teams is an explicit empty list", func() {
		BeforeEach(func() {
			desiredTeams = []v1alpha1.RepositoryTeamPermission{}
			repositoryTeams = []*github.Team{{
				Slug:       new("platform-team"),
				Permission: new("push"),
			}}
		})

		It("should remove all existing team permissions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(HaveLen(1))
			Expect(removeCalls[0].Slug).To(Equal("platform-team"))
		})
	})

	Context("when listing repository teams fails", func() {
		BeforeEach(func() {
			listRepoErr = errors.New("list repo teams failed")
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("list repo teams failed"))
		})
	})

	Context("when adding team permission fails", func() {
		BeforeEach(func() {
			addErr = errors.New("add team failed")
			desiredTeams = []v1alpha1.RepositoryTeamPermission{{
				TeamRef:    v1alpha1.TeamRef{Name: "platform-team"},
				Permission: "pull",
			}}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("add team failed"))
		})
	})

	Context("when removing team permission fails", func() {
		BeforeEach(func() {
			removeErr = errors.New("remove team failed")
			repositoryTeams = []*github.Team{{
				Slug:       new("platform-team"),
				Permission: new("pull"),
			}}
		})

		It("should return the error", func() {
			Expect(err).To(MatchError("remove team failed"))
		})
	})
})
