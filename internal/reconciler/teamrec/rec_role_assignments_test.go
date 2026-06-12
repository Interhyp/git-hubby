package teamrec

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

var _ = Describe("ReconcileTeamRoleAssignments", func() {
	var (
		ctx         context.Context
		mockClient1 *ghclientmock.MockGitHubClientWrapper
		mockClient2 *ghclientmock.MockGitHubClientWrapper
		k8sClient   client.Client
		rec         *GitHubTeamReconciler
		scheme      *runtime.Scheme
		team        *v1alpha1.Team
		err         error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient1 = ghclientmock.NewMockGitHubClientWrapper()
		mockClient2 = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		team = &v1alpha1.Team{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-team",
				Namespace: "default",
			},
			Spec: v1alpha1.TeamSpec{
				Name:    "test-team",
				Members: []string{"user1", "user2"},
				OrganizationRefs: []v1alpha1.OrganizationRef{
					{Name: "org1"},
				},
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(team).
			WithStatusSubresource(team).
			Build()
	})

	Context("when team has no organization roles specified", func() {
		BeforeEach(func() {
			// GetAllOrgRoles returns available roles
			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_read"), ID: new(int64(1))},
					{Name: new("all_repo_write"), ID: new(int64(2))},
					{Name: new("custom_role"), ID: new(int64(3))},
				}, nil
			}

			// Team is not yet assigned to any roles
			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should not add any roles", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			addCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(0))
		})
	})

	Context("when team already has roles assigned but none are specified", func() {
		BeforeEach(func() {
			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_write"), ID: new(int64(2))},
				}, nil
			}

			// Team is already assigned to the role
			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{"test-team"}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should remove the existing role assignment", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			addCalls := 0
			removeCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls++
				}
				if call.Method == "RemoveOrgRoleAssignmentForTeam" {
					removeCalls++
				}
			}
			Expect(addCalls).To(Equal(0))
			Expect(removeCalls).To(Equal(1))
		})
	})

	Context("when team has custom organization roles specified", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{"custom_role_1", "custom_role_2"}

			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_read"), ID: new(int64(1))},
					{Name: new("all_repo_write"), ID: new(int64(2))},
					{Name: new("custom_role_1"), ID: new(int64(3))},
					{Name: new("custom_role_2"), ID: new(int64(4))},
				}, nil
			}

			// Team is not yet assigned to custom roles
			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should add only the specified custom roles", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			addCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(2))
		})
	})

	Context("when team has roles that should be removed", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{"custom_role_1"}

			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("custom_role_1"), ID: new(int64(1))},
					{Name: new("custom_role_2"), ID: new(int64(2))},
					{Name: new("custom_role_3"), ID: new(int64(3))},
				}, nil
			}

			// Team is assigned to roles 1, 2, and 3, but spec only wants role 1
			callCount := 0
			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				callCount++
				if callCount <= 3 { // First call for each role during reconciliation
					return []string{"test-team"}, nil
				}
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should remove roles not specified in the spec", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			removeCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "RemoveOrgRoleAssignmentForTeam" {
					removeCalls++
				}
			}
			Expect(removeCalls).To(Equal(2)) // Should remove custom_role_2 and custom_role_3
		})
	})

	Context("when role specified in spec does not exist in organization", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{"non_existent_role", "custom_role_1"}

			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("custom_role_1"), ID: new(int64(1))},
				}, nil
			}

			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should only add the role that exists", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			addCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(1)) // Only custom_role_1 should be added
		})
	})

	Context("when reconciling across multiple organizations", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{"all_repo_write"}
			team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
				{Name: "org1"},
				{Name: "org2"},
			}

			// Setup for org1
			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_read"), ID: new(int64(1))},
					{Name: new("all_repo_write"), ID: new(int64(2))},
				}, nil
			}
			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			// Setup for org2
			mockClient2.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_read"), ID: new(int64(10))},
					{Name: new("all_repo_write"), ID: new(int64(20))},
				}, nil
			}
			mockClient2.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
							{
								Client:   mockClient2,
								Resource: "org2",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should assign roles in both organizations", func() {
			Expect(err).NotTo(HaveOccurred())

			roleAssignmentCalls1 := mockClient1.GetRoleAssignmentCalls()
			roleAssignmentCalls2 := mockClient2.GetRoleAssignmentCalls()

			addCalls1 := 0
			for _, call := range roleAssignmentCalls1 {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls1++
				}
			}

			addCalls2 := 0
			for _, call := range roleAssignmentCalls2 {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls2++
				}
			}

			Expect(addCalls1).To(Equal(1))
			Expect(addCalls2).To(Equal(1))
		})
	})

	Context("when GetAllOrgRoles fails", func() {
		BeforeEach(func() {
			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return nil, errors.New("failed to get org roles")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get org roles"))
		})
	})

	Context("when GetAllTeamsAssignedToOrgRole fails", func() {
		BeforeEach(func() {
			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_read"), ID: new(int64(1))},
				}, nil
			}

			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return nil, errors.New("failed to get role assignments")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get role assignments"))
		})
	})

	Context("when AddOrgRoleAssignmentForTeam fails", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{"all_repo_write"}

			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_write"), ID: new(int64(1))},
				}, nil
			}

			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			mockClient1.AddOrgRoleAssignmentForTeamFunc = func(ctx context.Context, org string, slug string, roleID int64) error {
				return errors.New("failed to add role assignment")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to add role assignment"))
		})
	})

	Context("when RemoveOrgRoleAssignmentForTeam fails", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{} // Empty spec, should remove all roles

			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("custom_role"), ID: new(int64(1))},
				}, nil
			}

			// Team is currently assigned to custom_role
			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{"test-team"}, nil
			}

			mockClient1.RemoveOrgRoleAssignmentForTeamFunc = func(ctx context.Context, org string, slug string, roleID int64) error {
				return errors.New("failed to remove role assignment")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to remove role assignment"))
		})
	})

	Context("when GetAllOrgRoles returns roles with nil names", func() {
		BeforeEach(func() {
			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("all_repo_read"), ID: new(int64(1))},
					{Name: nil, ID: new(int64(2))}, // nil name should be skipped
					{Name: new("all_repo_write"), ID: new(int64(3))},
					nil, // nil role should be skipped
				}, nil
			}

			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{

					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should skip nil roles and not add any roles", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			addCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(0))
		})
	})

	Context("when spec has empty organization roles list", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRoles = []string{}

			mockClient1.GetAllOrgRolesFunc = func(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{Name: new("some_role"), ID: new(int64(1))},
				}, nil
			}

			mockClient1.GetAllTeamsAssignedToOrgRoleFunc = func(ctx context.Context, org string, role string) ([]string, error) {
				return []string{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeamRoleAssignments(ctx)
		})

		It("should not assign any roles", func() {
			Expect(err).NotTo(HaveOccurred())
			roleAssignmentCalls := mockClient1.GetRoleAssignmentCalls()

			addCalls := 0
			for _, call := range roleAssignmentCalls {
				if call.Method == "AddOrgRoleAssignmentForTeam" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(0))
		})
	})
})

var _ = Describe("getRoles", func() {
	var (
		rec               *GitHubTeamReconciler
		orgRoleNamesToIDs map[string]int64
		result            []role
	)

	Context("when no custom organization roles are specified", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Resource: &v1alpha1.Team{
						Spec: v1alpha1.TeamSpec{
							OrganizationRoles: nil,
						},
					},
				},
			}

			orgRoleNamesToIDs = map[string]int64{
				"all_repo_read":  1,
				"all_repo_write": 2,
				"custom_role":    3,
			}

			result = rec.getRoles(orgRoleNamesToIDs)
		})

		It("should return empty list (no default roles)", func() {
			Expect(result).To(BeEmpty())
		})
	})

	Context("when custom organization roles are specified", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Resource: &v1alpha1.Team{
						Spec: v1alpha1.TeamSpec{
							OrganizationRoles: []string{"custom_role_1", "custom_role_2"},
						},
					},
				},
			}

			orgRoleNamesToIDs = map[string]int64{
				"all_repo_read":  1,
				"all_repo_write": 2,
				"custom_role_1":  3,
				"custom_role_2":  4,
			}

			result = rec.getRoles(orgRoleNamesToIDs)
		})

		It("should return only the specified custom roles", func() {
			Expect(result).To(HaveLen(2))
			Expect(result).To(ContainElement(role{Name: "custom_role_1", ID: 3}))
			Expect(result).To(ContainElement(role{Name: "custom_role_2", ID: 4}))
			Expect(result).NotTo(ContainElement(role{Name: "all_repo_read", ID: 1}))
			Expect(result).NotTo(ContainElement(role{Name: "all_repo_write", ID: 2}))
		})
	})

	Context("when specified role does not exist in org", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Resource: &v1alpha1.Team{
						Spec: v1alpha1.TeamSpec{
							OrganizationRoles: []string{"non_existent_role", "custom_role_1"},
						},
					},
				},
			}

			orgRoleNamesToIDs = map[string]int64{
				"custom_role_1": 1,
				"custom_role_2": 2,
			}

			result = rec.getRoles(orgRoleNamesToIDs)
		})

		It("should return only roles that exist in the org", func() {
			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(role{Name: "custom_role_1", ID: 1}))
			Expect(result).NotTo(ContainElement(role{Name: "non_existent_role", ID: 0}))
		})
	})

	Context("when empty organization roles list is specified", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Resource: &v1alpha1.Team{
						Spec: v1alpha1.TeamSpec{
							OrganizationRoles: []string{},
						},
					},
				},
			}

			orgRoleNamesToIDs = map[string]int64{
				"all_repo_read":  1,
				"all_repo_write": 2,
			}

			result = rec.getRoles(orgRoleNamesToIDs)
		})

		It("should return empty list (no roles)", func() {
			Expect(result).To(BeEmpty())
		})
	})

	Context("when orgRoleNamesToIDs is empty", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Resource: &v1alpha1.Team{
						Spec: v1alpha1.TeamSpec{
							OrganizationRoles: nil,
						},
					},
				},
			}

			orgRoleNamesToIDs = map[string]int64{}

			result = rec.getRoles(orgRoleNamesToIDs)
		})

		It("should return empty list", func() {
			Expect(result).To(BeEmpty())
		})
	})
})
