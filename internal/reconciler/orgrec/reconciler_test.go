package orgrec

import (
	"context"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileDeletion", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				Description:             "Test Organization",
				GitHubAppInstallationId: 12345,
			},
		}
	})

	JustBeforeEach(func() {
		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.ReconcileDeletion(ctx)
	})

	Context("when organization has no repositories or teams", func() {
		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org).
				WithStatusSubresource(org).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					orgNames := make([]string, 0, len(team.Spec.OrganizationRefs))
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should allow deletion and return no error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when organization has repositories", func() {
		BeforeEach(func() {
			repo := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: "test-org",
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, repo).
				WithStatusSubresource(org, repo).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should prevent deletion and return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(FinalizationFailedError{}))
			Expect(err.Error()).To(Equal("organization still has repositories, cannot delete it"))
		})
	})

	Context("when organization has multiple repositories", func() {
		BeforeEach(func() {
			repo1 := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo-1",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo-1",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: "test-org",
					},
				},
			}

			repo2 := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo-2",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo-2",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: "test-org",
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, repo1, repo2).
				WithStatusSubresource(org, repo1, repo2).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should prevent deletion and return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(FinalizationFailedError{}))
			Expect(err.Error()).To(Equal("organization still has repositories, cannot delete it"))
		})
	})

	Context("when organization has teams", func() {
		BeforeEach(func() {
			team := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team",
					Members: []string{"user1", "user2"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "test-org"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, team).
				WithStatusSubresource(org, team).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should prevent deletion and return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(FinalizationFailedError{}))
			Expect(err.Error()).To(Equal("organization still has teams, cannot delete it"))
		})
	})

	Context("when organization has multiple teams", func() {
		BeforeEach(func() {
			team1 := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team-1",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team-1",
					Members: []string{"user1"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "test-org"},
					},
				},
			}

			team2 := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team-2",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team-2",
					Members: []string{"user2"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "test-org"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, team1, team2).
				WithStatusSubresource(org, team1, team2).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should prevent deletion and return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(FinalizationFailedError{}))
			Expect(err.Error()).To(Equal("organization still has teams, cannot delete it"))
		})
	})

	Context("when organization has both repositories and teams", func() {
		BeforeEach(func() {
			repo := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: "test-org",
					},
				},
			}

			team := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team",
					Members: []string{"user1"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "test-org"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, repo, team).
				WithStatusSubresource(org, repo, team).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should prevent deletion and return error about repositories first", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(FinalizationFailedError{}))
			// The method checks repositories first, so this error should be returned
			Expect(err.Error()).To(Equal("organization still has repositories, cannot delete it"))
		})
	})

	Context("when organization has repositories in a different namespace", func() {
		BeforeEach(func() {
			repoInDifferentNamespace := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "other-namespace",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: "test-org",
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, repoInDifferentNamespace).
				WithStatusSubresource(org, repoInDifferentNamespace).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should allow deletion (namespace scoped)", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when organization has teams in a different namespace", func() {
		BeforeEach(func() {
			teamInDifferentNamespace := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "other-namespace",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team",
					Members: []string{"user1"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "test-org"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, teamInDifferentNamespace).
				WithStatusSubresource(org, teamInDifferentNamespace).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should allow deletion (namespace scoped)", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when organization has repositories referencing a different organization", func() {
		BeforeEach(func() {
			repoForDifferentOrg := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: "different-org",
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, repoForDifferentOrg).
				WithStatusSubresource(org, repoForDifferentOrg).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should allow deletion", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when organization has teams referencing multiple organizations (including this one)", func() {
		BeforeEach(func() {
			teamInMultipleOrgs := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team",
					Members: []string{"user1"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "test-org"},
						{Name: "other-org"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, teamInMultipleOrgs).
				WithStatusSubresource(org, teamInMultipleOrgs).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should prevent deletion and return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(FinalizationFailedError{}))
			Expect(err.Error()).To(Equal("organization still has teams, cannot delete it"))
		})
	})

	Context("when organization has teams referencing only other organizations", func() {
		BeforeEach(func() {
			teamInOtherOrgs := &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name:    "test-team",
					Members: []string{"user1"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "other-org"},
						{Name: "another-org"},
					},
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, teamInOtherOrgs).
				WithStatusSubresource(org, teamInOtherOrgs).
				WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
					repo := obj.(*v1alpha1.Repository)
					return []string{repo.Spec.OrganizationRef.Name}
				}).
				WithIndex(&v1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
					team := obj.(*v1alpha1.Team)
					var orgNames []string
					for _, orgRef := range team.Spec.OrganizationRefs {
						orgNames = append(orgNames, orgRef.Name)
					}
					return orgNames
				}).
				Build()
		})

		It("should allow deletion", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("RequiredReconciliations", func() {
	var (
		rec *GitHubOrgReconciler
		org *v1alpha1.Organization
	)

	BeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				Description:             "Test Organization",
				GitHubAppInstallationId: 12345,
			},
		}
	})

	JustBeforeEach(func() {
		rec = &GitHubOrgReconciler{
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Resource: org,
			},
		}
	})

	It("should return all reconcilers in a single parallel group regardless of plan", func() {
		groups := rec.RequiredReconciliations()
		Expect(groups).To(HaveLen(1))
		// All reconcilers run in parallel; plan-based checks are handled within each reconciler
		Expect(groups[0]).To(HaveLen(5))
	})
})
