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

// Simplified Repository Controller Integration Tests
// Focus on integration-level concerns: K8s API, controller-runtime mechanics, end-to-end flow
// Detailed CRUD, webhook, ruleset, and actions logic is covered by unit tests in:
// - internal/reconciler/reporec/*_test.go (repository CRUD, webhooks, rulesets, actions, custom properties)
// - internal/reconciler/reconcilerfactory/factory_test.go (factory creation logic)
// - internal/reconciler/executor_test.go (label management, condition updates, concurrent execution)

var _ = Describe("Repository Controller - Integration Tests", func() {
	var (
		testEnv      *TestEnvironment
		mockClient   *ghclientmock.MockGitHubClientWrapper
		factory      *reconcilerfactory.Factory
		organization *githubv1alpha1.Organization
	)

	const (
		repoName      = "test-repo"
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
			LegacySecretName: secretName,
		}
		testEnv.CreateTestNamespace(namespaceName)
		testEnv.CreateSecret(namespaceName, secretName)
		organization = testEnv.SetupOrganizationTest(nil, namespaceName, orgName)
		organization.Spec.ActionsSettings.EnabledRepositories = new("all")
		Expect(testEnv.Client.Update(testEnv.Context, organization)).To(Succeed())
	})

	AfterEach(func() {
		testEnv.TeardownTestEnvironment()
	})

	Context("Basic reconciliation flow", func() {
		var (
			repository     *githubv1alpha1.Repository
			namespacedName types.NamespacedName
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      repoName,
				Namespace: namespaceName,
			}
			repository = testEnv.SetupRepositoryTest(nil, namespaceName, repoName, orgName)
		})

		AfterEach(func() {
			if repository != nil {
				testEnv.CleanupTestResources(repository)
			}
		})

		It("should successfully reconcile a repository through factory and executor", func() {
			By("Setting up mock to return existing repository")
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, repo string) (*github.Repository, error) {
				return &github.Repository{
					ID:         new(int64(12345)),
					Name:       github.Ptr(repoName),
					FullName:   new(owner + "/" + repo),
					Owner:      &github.User{Login: new(owner)},
					Archived:   new(false),
					Visibility: new("internal"),
				}, nil
			}

			By("Creating reconciler from factory")
			repoReconciler, err := factory.CreateForRepo(ctx, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(repoReconciler).NotTo(BeNil())

			By("Reconciling the repository")
			err = repoReconciler.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying GitHub API was called")
			Expect(mockClient.GetRepositoryCalls()).NotTo(BeEmpty())
		})

		It("should handle organization reference not found", func() {
			By("Deleting the organization")
			Expect(testEnv.Client.Delete(testEnv.Context, organization)).To(Succeed())

			By("Creating reconciler from factory should fail")
			_, err := factory.CreateForRepo(ctx, namespacedName)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Repository deletion with finalizer", func() {
		var (
			repository     *githubv1alpha1.Repository
			namespacedName types.NamespacedName
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      repoName,
				Namespace: namespaceName,
			}
			repository = testEnv.SetupRepositoryTest(nil, namespaceName, repoName, orgName)
			repository.Finalizers = []string{"repository.github.interhyp.de/finalizer"}
			Expect(testEnv.Client.Update(testEnv.Context, repository)).To(Succeed())
			Expect(testEnv.Client.Delete(testEnv.Context, repository)).To(Succeed())
		})

		AfterEach(func() {
			if repository != nil {
				testEnv.CleanupTestResources(repository)
			}
		})

		It("should archive repository when marked for deletion", func() {
			By("Setting up mock to return unarchived repository")
			mockClient.SetRepositoryArchived(orgName, repoName, false)

			By("Creating reconciler from factory")
			repoReconciler, err := factory.CreateForRepo(ctx, namespacedName)
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the repository")
			_ = repoReconciler.Reconcile(testEnv.Context)

			By("Verifying repository archive was attempted")
			Expect(mockClient.GetRepositoryCalls()).To(ContainElement(HaveField("Method", Equal("EditRepository"))))
		})
	})

	Context("Status updates through K8s client", func() {
		var (
			repository     *githubv1alpha1.Repository
			namespacedName types.NamespacedName
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      repoName,
				Namespace: namespaceName,
			}
			repository = testEnv.SetupRepositoryTest(nil, namespaceName, repoName, orgName)
		})

		AfterEach(func() {
			if repository != nil {
				testEnv.CleanupTestResources(repository)
			}
		})

		It("should update repository status with GitHub ID", func() {
			By("Setting up mock to return repository with ID")
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, repo string) (*github.Repository, error) {
				return &github.Repository{
					ID:                  new(int64(99999)),
					Name:                github.Ptr(repoName),
					FullName:            new(owner + "/" + repo),
					Owner:               &github.User{Login: new(owner)},
					Archived:            new(false),
					Visibility:          new("internal"),
					HasIssues:           new(true),
					HasProjects:         new(false),
					HasWiki:             new(false),
					HasDownloads:        new(false),
					IsTemplate:          new(false),
					AutoInit:            new(true),
					AllowSquashMerge:    new(false),
					AllowRebaseMerge:    new(false),
					AllowMergeCommit:    new(false),
					DeleteBranchOnMerge: new(true),
					MergeCommitTitle:    new("MERGE_MESSAGE"),
					MergeCommitMessage:  new("PR_TITLE"),
					Homepage:            new(""),
					Description:         new(""),
					DefaultBranch:       new(""),
				}, nil
			}

			By("Creating reconciler and reconciling")
			repoReconciler, err := factory.CreateForRepo(ctx, namespacedName)
			Expect(err).NotTo(HaveOccurred())
			err = repoReconciler.Reconcile(testEnv.Context)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying status was updated")
			var updatedRepo githubv1alpha1.Repository
			err = testEnv.Client.Get(testEnv.Context, namespacedName, &updatedRepo)
			Expect(err).NotTo(HaveOccurred())
			Expect(*updatedRepo.Status.ID).To(Equal(int64(99999)))
		})
	})
})
