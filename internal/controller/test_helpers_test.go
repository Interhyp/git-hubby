package controller

import (
	"context"
	"os"
	"testing"

	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	applyconfiguration "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration"
)

// TestEnvironment holds the test environment state for unit testing
type TestEnvironment struct {
	Client  client.Client
	Context context.Context
}

// SetupTestEnvironment initializes the test environment using fake client
func SetupTestEnvironment() *TestEnvironment {
	ctx := context.Background()

	// Add our scheme to the default scheme
	err := githubv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	// Create a fake client for testing with proper scheme, status subresource, and SSA type converter
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithTypeConverters(
			applyconfiguration.NewTypeConverter(scheme.Scheme),
			managedfields.NewDeducedTypeConverter(),
		).
		WithStatusSubresource(&githubv1alpha1.Repository{}).
		WithStatusSubresource(&githubv1alpha1.Team{}).
		WithStatusSubresource(&githubv1alpha1.Organization{}).
		WithIndex(&githubv1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
			repo := obj.(*githubv1alpha1.Repository)
			return []string{repo.Spec.OrganizationRef.Name}
		}).
		WithIndex(&githubv1alpha1.Team{}, "spec.organizationRefs.name", func(obj client.Object) []string {
			team := obj.(*githubv1alpha1.Team)
			orgNames := make([]string, 0, len(team.Spec.OrganizationRefs))
			for _, orgRef := range team.Spec.OrganizationRefs {
				orgNames = append(orgNames, orgRef.Name)
			}
			return orgNames
		}).
		Build()

	_ = os.Setenv("REPOSITORY_FINALIZER_MODE", string(reconciler.Archive))

	return &TestEnvironment{
		Client:  fakeClient,
		Context: ctx,
	}
}

// TeardownTestEnvironment cleans up the test environment
func (te *TestEnvironment) TeardownTestEnvironment() {
	// Nothing to tear down for unit tests with fake client
	_ = os.Unsetenv("REPOSITORY_FINALIZER_MODE")
}

// CreateMockGitHubRepository creates a mock GitHub repository with common test scenarios
func CreateMockGitHubRepository() *ghclientmock.MockGitHubClientWrapper {
	return ghclientmock.NewMockGitHubClientWrapper()
}

// SetupTeamTest creates a test team resource
func (te *TestEnvironment) SetupTeamTest(_ *testing.T, namespace, teamName string, teamSlug *string, organizationRefs []githubv1alpha1.OrganizationRef) *githubv1alpha1.Team {
	// Create namespace first
	ns := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: namespace,
		},
	}
	_ = te.Client.Create(te.Context, ns) // Ignore error if namespace already exists

	// Create the Team
	team := &githubv1alpha1.Team{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      teamName,
			Namespace: namespace,
		},
		Spec: githubv1alpha1.TeamSpec{
			Name:             teamName,
			OrganizationRefs: organizationRefs,
			Members:          []string{"someone"},
		},
		Status: githubv1alpha1.TeamStatus{
			Slug: teamSlug,
		},
	}

	Expect(te.Client.Create(te.Context, team)).To(Succeed())
	return team
}

func (te *TestEnvironment) SetupTeamWithMembersTest(_ *testing.T, namespace, teamName, orgName string) *githubv1alpha1.Team {
	// Create namespace first
	ns := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: namespace,
		},
	}
	_ = te.Client.Create(te.Context, ns) // Ignore error if namespace already exists

	// Create the Team
	team := &githubv1alpha1.Team{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      teamName,
			Namespace: namespace,
		},
		Spec: githubv1alpha1.TeamSpec{
			Name: teamName,
			OrganizationRefs: []githubv1alpha1.OrganizationRef{
				{Name: orgName},
			},
			Members: []string{"new-member", "existing-member"},
		},
		Status: githubv1alpha1.TeamStatus{
			Slug: new(teamName),
		},
	}

	Expect(te.Client.Create(te.Context, team)).To(Succeed())
	return team
}

// SetupOrganizationTest creates a test Organization resource
func (te *TestEnvironment) SetupOrganizationTest(_ *testing.T, namespace, orgName string) *githubv1alpha1.Organization {
	// Create namespace first
	ns := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: namespace,
		},
	}
	_ = te.Client.Create(te.Context, ns) // Ignore error if namespace already exists

	// Create the Organization
	org := &githubv1alpha1.Organization{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      orgName,
			Namespace: namespace,
		},
		Spec: githubv1alpha1.OrganizationSpec{
			Name:        orgName,
			Description: "Test organization for unit tests",
		},
	}

	Expect(te.Client.Create(te.Context, org)).To(Succeed())
	return org
}

// SetupRepositoryTest creates a test repository resource
func (te *TestEnvironment) SetupRepositoryTest(_ *testing.T, namespace, repoName, orgName string) *githubv1alpha1.Repository {
	// Create namespace first
	ns := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: namespace,
		},
	}
	_ = te.Client.Create(te.Context, ns) // Ignore error if namespace already exists

	// Create the repository
	repo := &githubv1alpha1.Repository{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      repoName,
			Namespace: namespace,
		},
		Spec: githubv1alpha1.RepositorySpec{
			Name: repoName,
			OrganizationRef: githubv1alpha1.OrganizationRef{
				Name: orgName,
			},
			// Default value for tests
			Archived:            new(false),
			Visibility:          "internal",
			HasIssues:           new(true),
			HasProjects:         new(false),
			HasWiki:             new(false),
			HasDownloads:        new(false),
			IsTemplate:          new(false),
			DeleteBranchOnMerge: new(true),
			MergeCommitMessage:  "PR_TITLE",
			MergeCommitTitle:    "MERGE_MESSAGE",

			// Initialize empty webhook preset list to avoid webhook processing
			WebhookPresetList: []corev1.LocalObjectReference{},
		},
		Status: githubv1alpha1.RepositoryStatus{
			Webhooks: map[string]githubv1alpha1.WebhookStatus{},
		},
	}

	Expect(te.Client.Create(te.Context, repo)).To(Succeed())
	return repo
}

// CreateTestNamespace creates a namespace for testing
func (te *TestEnvironment) CreateTestNamespace(namespaceName string) {
	namespace := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: namespaceName,
		},
	}

	_ = te.Client.Create(te.Context, namespace) // Ignore error if namespace already exists
}

// CleanupTestResources removes test resources
func (te *TestEnvironment) CleanupTestResources(resources ...client.Object) {
	for _, resource := range resources {
		_ = te.Client.Delete(te.Context, resource) // Ignore errors in cleanup
	}
}

// CreateSecret creates a test GitHub App credentials secret
func (te *TestEnvironment) CreateSecret(namespace, secretName string) {
	secret := &corev1.Secret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"app-id":              []byte("12345"),
			"app-installation-id": []byte("67890"),
			"private-key": []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA2rjjn+XfxEK+8X+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j
3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8
qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9J
Z8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+
X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3
nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8q
J9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ
8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X
3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3n
V8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ
9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8
V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3
j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV
8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9
JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V
+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j
3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8qJ9JZ8V+X3j3nV8
qJ9JZ8QIDAQAB
-----END RSA PRIVATE KEY-----`),
		},
	}
	Expect(te.Client.Create(te.Context, secret)).To(Succeed())
}
