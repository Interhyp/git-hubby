/*
Copyright 2025.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"
	"github.com/Interhyp/git-hubby/test/mock"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

// Simplified Organization Controller Integration Tests
// Focus on integration-level concerns: K8s API, controller-runtime mechanics, end-to-end flow
// Detailed CRUD, custom properties, rulesets, actions, code security, and webhook logic is covered by unit tests in:
// - internal/reconciler/orgrec/*_test.go (organization CRUD, custom properties, rulesets, actions, code security configurations)
// - internal/reconciler/reconcilerfactory/factory_test.go (factory creation logic)
// - internal/reconciler/executor_test.go (label management, condition updates, concurrent execution)
var _ = Describe("Organization Controller - Integration Tests", func() {
	var (
		testEnv    *TestEnvironment
		mockClient *ghclientmock.MockGitHubClientWrapper
		factory    *reconcilerfactory.Factory
	)
	const (
		orgName       = "test-org"
		namespaceName = "test-namespace"
		secretName    = "github-app-credentials"
	)
	BeforeEach(func() {
		testEnv = SetupTestEnvironment()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		factory = &reconcilerfactory.Factory{
			ClientManager:    ghclientmock.NewGitHubMockClientFactory(mockClient),
			K8sClient:        testEnv.Client,
			SpreadingManager: &mock.NoOpSpreadManager{},
		}
		// Create test namespace and secret
		testEnv.CreateTestNamespace(namespaceName)
		testEnv.CreateSecret(namespaceName, secretName)
	})
	AfterEach(func() {
		testEnv.TeardownTestEnvironment()
	})
	Context("Basic reconciliation flow", func() {
		var (
			organization   *githubv1alpha1.Organization
			namespacedName types.NamespacedName
		)
		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      orgName,
				Namespace: namespaceName,
			}
			organization = testEnv.SetupOrganizationTest(nil, namespaceName, orgName)
		})
		AfterEach(func() {
			testEnv.CleanupTestResources(organization)
		})
		It("should successfully reconcile an organization through factory and executor", func() {
			By("Setting up mock to return existing organization")
			mockClient.GetOrganizationFunc = func(ctx context.Context, org string) (*github.Organization, error) {
				return &github.Organization{
					Login:       github.Ptr(orgName),
					Name:        github.Ptr(orgName),
					Description: github.Ptr("Test organization for unit tests"),
				}, nil
			}
			mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{}, nil
			}
			By("Creating reconciler from factory")
			orgReconciler, err := factory.CreateForOrg(testEnv.Context, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(orgReconciler).NotTo(BeNil())
			By("Reconciling the organization")
			err = orgReconciler.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())
			By("Verifying GitHub API was called")
			Expect(mockClient.GetOrganizationCalls()).NotTo(BeEmpty())
		})
		It("should attach finalizer during reconciliation", func() {
			By("Creating organization without finalizer")
			testEnv.CleanupTestResources(organization)
			orgWithoutFinalizer := testEnv.SetupOrganizationTest(nil, namespaceName, orgName)
			// Remove finalizer manually to simulate missing finalizer
			orgWithoutFinalizer.Finalizers = []string{}
			Expect(testEnv.Client.Update(testEnv.Context, orgWithoutFinalizer)).To(Succeed())
			By("Setting up mock to return existing organization")
			mockClient.GetOrganizationFunc = func(ctx context.Context, org string) (*github.Organization, error) {
				return &github.Organization{
					Login:       github.Ptr(orgName),
					Name:        github.Ptr(orgName),
					Description: github.Ptr("Test organization for unit tests"),
				}, nil
			}
			mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{}, nil
			}
			By("Creating reconciler from factory")
			orgReconciler, err := factory.CreateForOrg(testEnv.Context, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			By("Reconciling the organization")
			err = orgReconciler.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())
			By("Verifying finalizer was added")
			var updatedOrg githubv1alpha1.Organization
			Expect(testEnv.Client.Get(testEnv.Context, namespacedName, &updatedOrg)).To(Succeed())
			Expect(updatedOrg.Finalizers).To(ContainElement("organization.github.interhyp.de/finalizer"))
			organization = orgWithoutFinalizer
		})
	})
	Context("Organization deletion with finalizer", func() {
		var (
			organization   *githubv1alpha1.Organization
			namespacedName types.NamespacedName
		)
		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      orgName,
				Namespace: namespaceName,
			}
			organization = testEnv.SetupOrganizationTest(nil, namespaceName, orgName)
		})
		AfterEach(func() {
			testEnv.CleanupTestResources(organization)
		})
		It("should successfully finalize when no repositories and teams exist", func() {
			By("Creating reconciler from factory first (before deletion)")
			mockClient.GetOrganizationFunc = func(ctx context.Context, org string) (*github.Organization, error) {
				return &github.Organization{
					Login:       github.Ptr(orgName),
					Name:        github.Ptr(orgName),
					Description: github.Ptr("Test organization for unit tests"),
				}, nil
			}
			mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{}, nil
			}
			orgReconciler, err := factory.CreateForOrg(testEnv.Context, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			By("Verifying initial reconciliation attaches finalizer")
			err = orgReconciler.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())
			By("Verifying finalizer was attached")
			var updatedOrg githubv1alpha1.Organization
			Expect(testEnv.Client.Get(testEnv.Context, namespacedName, &updatedOrg)).To(Succeed())
			Expect(updatedOrg.Finalizers).To(ContainElement("organization.github.interhyp.de/finalizer"))
			By("Marking organization for deletion")
			Expect(testEnv.Client.Delete(testEnv.Context, &updatedOrg)).To(Succeed())
			By("Creating new reconciler for deleted organization")
			orgReconciler2, err := factory.CreateForOrg(testEnv.Context, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			By("Reconciling the organization (should handle finalization)")
			err = orgReconciler2.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Status updates through K8s client", func() {
		var (
			organization   *githubv1alpha1.Organization
			namespacedName types.NamespacedName
		)
		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      orgName,
				Namespace: namespaceName,
			}
			organization = testEnv.SetupOrganizationTest(nil, namespaceName, orgName)
		})
		AfterEach(func() {
			testEnv.CleanupTestResources(organization)
		})
		It("should update organization status conditions after reconciliation", func() {
			By("Setting up mock to return existing organization")
			mockClient.GetOrganizationFunc = func(ctx context.Context, org string) (*github.Organization, error) {
				return &github.Organization{
					Login:       github.Ptr(orgName),
					Name:        github.Ptr(orgName),
					Description: github.Ptr("Test organization"),
				}, nil
			}
			mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{}, nil
			}
			By("Creating reconciler and reconciling")
			orgReconciler, err := factory.CreateForOrg(testEnv.Context, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			err = orgReconciler.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())
			By("Verifying status conditions were updated")
			var updatedOrg githubv1alpha1.Organization
			err = testEnv.Client.Get(testEnv.Context, namespacedName, &updatedOrg)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedOrg.Status.Conditions).NotTo(BeEmpty())
		})
	})
})
