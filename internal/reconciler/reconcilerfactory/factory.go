package reconcilerfactory

import (
	"context"
	"fmt"
	"os"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/orgrec"
	"github.com/Interhyp/git-hubby/internal/reconciler/reporec"
	"github.com/Interhyp/git-hubby/internal/reconciler/teamrec"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

type Factory struct {
	ClientManager    reconciler.GitHubClientManager
	K8sClient        client.Client
	SpreadingManager reconciler.SpreadManager
	// LegacySecretName is the name of the credential secret used for Organizations that
	// still rely on the deprecated GitHubAppInstallationId field without a GitHubAppConfig.
	LegacySecretName string
}

const (
	orgRateLimitThreshold  = 100
	repoRateLimitThreshold = 100
	teamRateLimitThreshold = 100
)

// CreateForOrg creates a reconciler.ReconciliationExecutor for a v1alpha1.Organization
// If both the returned Executor and the error are nil, the Organization K8s resource was not found and no reconciliation is necessary.
func (f *Factory) CreateForOrg(ctx context.Context, namespacedOrgName types.NamespacedName) (*reconciler.ReconciliationExecutor[*v1alpha1.Organization], error) {
	log := logPkg.FromContext(ctx)

	var org v1alpha1.Organization
	if err := f.K8sClient.Get(ctx, namespacedOrgName, &org); err != nil {
		notFoundIgnored := client.IgnoreNotFound(err)
		if notFoundIgnored != nil {
			log.Error(notFoundIgnored, "unable to fetch Organization")
		}
		return nil, notFoundIgnored
	}

	// Log deprecation warning if using legacy name-only mode
	if org.IsUsingLegacyNameField() {
		log.Info("DEPRECATED: Organization uses 'name' field without explicit 'login' field. Consider setting 'login' to separate login from display name",
			"organization", org.Name,
			"effectiveLogin", org.GetLogin())
	}

	subResourceGenerations, err := f.fetchSubResourceGenerationsForOrg(ctx, org)
	if err != nil {
		return nil, err
	}

	if requiresSpreadErr := f.SpreadingManager.Spread(ctx, &org, subResourceGenerations); requiresSpreadErr != nil {
		return nil, requiresSpreadErr
	}

	appConfig, err := org.ResolveGitHubAppConfig(f.LegacySecretName)
	if err != nil {
		logPkg.FromContext(ctx).Error(err, "unable to resolve GitHub App config for Organization")
		return nil, err
	}

	ghClient, err := f.ClientManager.GetGitHubClientAndCheckRateLimit(ctx, org.GetLogin(), *appConfig, orgRateLimitThreshold)
	if err != nil {
		return nil, err
	}
	return &reconciler.ReconciliationExecutor[*v1alpha1.Organization]{
		Reconciler: &orgrec.GitHubOrgReconciler{
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:                        f.K8sClient,
				Resource:                      &org,
				CurrentSubResourceGenerations: subResourceGenerations,
			},
			GitHub: reconciler.GitHub[string]{
				Client:   ghClient,
				Resource: org.GetLogin(),
			},
		},
	}, nil
}

// CreateForRepo creates a reconciler.ReconciliationExecutor for a v1alpha1.Repository
// If both the returned Executor and the error are nil, the Repository K8s resource was not found and no reconciliation is necessary.
func (f *Factory) CreateForRepo(ctx context.Context, repoName types.NamespacedName) (*reconciler.ReconciliationExecutor[*v1alpha1.Repository], error) {
	log := logPkg.FromContext(ctx)

	var repo v1alpha1.Repository
	if err := f.K8sClient.Get(ctx, repoName, &repo); err != nil {
		notFoundIgnored := client.IgnoreNotFound(err)
		if notFoundIgnored != nil {
			log.Error(notFoundIgnored, "unable to fetch Repository")
		}
		return nil, notFoundIgnored
	}

	subResourceGenerations, err := f.fetchSubResourceGenerationsForRepo(ctx, repo)
	if err != nil {
		return nil, err
	}

	if requiresSpreadErr := f.SpreadingManager.Spread(ctx, &repo, subResourceGenerations); requiresSpreadErr != nil {
		return nil, requiresSpreadErr
	}

	org, err := f.getOrgByRef(ctx, repo.Spec.OrganizationRef.Name, repo.Namespace)
	if err != nil {
		log.Error(err, "unable to fetch Organization for Repository", "organization", repo.Spec.OrganizationRef.Name)
		return nil, err
	}

	appConfig, err := org.ResolveGitHubAppConfig(f.LegacySecretName)
	if err != nil {
		log.Error(err, "unable to resolve GitHub App config for Organization", "organization", org.GetLogin())
		return nil, err
	}

	ghClient, err := f.ClientManager.GetGitHubClientAndCheckRateLimit(ctx, org.GetLogin(), *appConfig, repoRateLimitThreshold)
	if err != nil {
		return nil, err
	}
	return &reconciler.ReconciliationExecutor[*v1alpha1.Repository]{
		Reconciler: &reporec.GitHubRepoReconciler{
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:                        f.K8sClient,
				Resource:                      &repo,
				CurrentSubResourceGenerations: subResourceGenerations,
			},
			GitHub: reconciler.GitHub[reporec.GitHubRepoIdentifier]{
				Client: ghClient,
				Resource: reporec.GitHubRepoIdentifier{
					Owner: org.GetLogin(),
					Name:  repo.Spec.Name,
				},
			},
			FinalizeMode: reconciler.RepositoryFinalizerMode(os.Getenv("REPOSITORY_FINALIZER_MODE")),
		},
	}, nil
}

// CreateForTeam creates a reconciler.ReconciliationExecutor for a v1alpha1.Team
// If both the returned Executor and the error are nil, the Team K8s resource was not found and no reconciliation is necessary.
func (f *Factory) CreateForTeam(ctx context.Context, teamName types.NamespacedName) (*reconciler.ReconciliationExecutor[*v1alpha1.Team], error) {
	var team v1alpha1.Team
	if err := f.K8sClient.Get(ctx, teamName, &team); err != nil {
		notFoundIgnored := client.IgnoreNotFound(err)
		if notFoundIgnored != nil {
			logPkg.FromContext(ctx).Error(notFoundIgnored, "unable to fetch Team")
		}
		return nil, notFoundIgnored
	}

	if requiresSpreadErr := f.SpreadingManager.Spread(ctx, &team, nil); requiresSpreadErr != nil {
		return nil, requiresSpreadErr
	}

	currentOrgs, err := buildGitHubOrgsSlice(ctx, f, team, func(t v1alpha1.Team) []v1alpha1.OrganizationRef {
		return t.Spec.OrganizationRefs
	})
	if err != nil {
		return nil, err
	}
	if len(currentOrgs) == 0 {
		return nil, fmt.Errorf("no organizations found for Team %s/%s", team.Namespace, team.Name)
	}

	previousOrgs, err := buildGitHubOrgsSlice(ctx, f, team, func(t v1alpha1.Team) []v1alpha1.OrganizationRef {
		return t.Status.PreviousOrganizationRefs
	})
	if err != nil {
		return nil, err
	}
	return &reconciler.ReconciliationExecutor[*v1alpha1.Team]{
		Reconciler: &teamrec.GitHubTeamReconciler{
			Team: reconciler.GitHubTeamIdentifier{
				Name: team.Spec.Name,
				Slug: team.Status.Slug,
				Organizations: reconciler.ReferencedOrganizations{
					Current:  currentOrgs,
					Previous: previousOrgs,
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
				Client:   f.K8sClient,
				Resource: &team,
			},
		},
	}, nil
}

func (f *Factory) getOrgByRef(ctx context.Context, orgRef, namespace string) (v1alpha1.Organization, error) {
	log := logPkg.FromContext(ctx)

	var org v1alpha1.Organization
	if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: orgRef, Namespace: namespace}, &org); err != nil {
		log.Error(err, "unable to fetch Organization for Team", "organization", orgRef)
		return org, err
	}
	return org, nil
}

func (f *Factory) fetchSubResourceGenerationsForOrg(ctx context.Context, org v1alpha1.Organization) (map[string]int64, error) {
	log := logPkg.FromContext(ctx).WithValues("method", "fetchSubResourceGenerationsForOrg")

	result := make(map[string]int64)

	for _, ruleSetPresetRef := range org.Spec.RulesetPresetList {
		var rulesetPreset v1alpha1.RulesetPreset
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: ruleSetPresetRef.Name, Namespace: org.Namespace}, &rulesetPreset); err != nil {
			log.Error(err, "failed to get RuleSetPreset for Organization")
			return nil, err
		}
		result[getSubResourceKey(&rulesetPreset)] = rulesetPreset.GetGeneration()
	}
	for _, cscRef := range org.Spec.CodeSecurityConfigurations {
		var csc v1alpha1.CodeSecurityConfiguration
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: cscRef.Name, Namespace: org.Namespace}, &csc); err != nil {
			log.Error(err, "failed to get CodeSecurityConfiguration for Organization")
			return nil, err
		}
		result[getSubResourceKey(&csc)] = csc.GetGeneration()
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func (f *Factory) fetchSubResourceGenerationsForRepo(ctx context.Context, repo v1alpha1.Repository) (map[string]int64, error) {
	log := logPkg.FromContext(ctx).WithValues("method", "fetchSubResourceGenerationsForRepo")

	result := make(map[string]int64)

	for _, ruleSetPresetRef := range repo.Spec.RulesetPresetList {
		var rulesetPreset v1alpha1.RulesetPreset
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: ruleSetPresetRef.Name, Namespace: repo.Namespace}, &rulesetPreset); err != nil {
			log.Error(err, "failed to get RuleSetPreset for Repository")
			return nil, err
		}
		result[getSubResourceKey(&rulesetPreset)] = rulesetPreset.GetGeneration()
	}
	for _, webhookPresetRefs := range repo.Spec.WebhookPresetList {
		var webhookPreset v1alpha1.WebhookPreset
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: webhookPresetRefs.Name, Namespace: repo.Namespace}, &webhookPreset); err != nil {
			log.Error(err, "failed to get WebhookPreset for Repository")
			return nil, err
		}
		result[getSubResourceKey(&webhookPreset)] = webhookPreset.GetGeneration()
	}
	for _, webhookIgnoreRefs := range repo.Spec.WebhookIgnorePresetsList {
		var webhookIgnorePreset v1alpha1.WebhookIgnorePreset
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: webhookIgnoreRefs.Name, Namespace: repo.Namespace}, &webhookIgnorePreset); err != nil {
			log.Error(err, "failed to get WebhookIgnorePreset for Repository")
			return nil, err
		}
		result[getSubResourceKey(&webhookIgnorePreset)] = webhookIgnorePreset.GetGeneration()
	}
	if repo.Spec.AttachedCodeSecurityConfiguration != nil {
		var csc v1alpha1.CodeSecurityConfiguration
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: repo.Spec.AttachedCodeSecurityConfiguration.Name, Namespace: repo.Namespace}, &csc); err != nil {
			log.Error(err, "failed to get AttachedCodeSecurityConfiguration for Repository")
			return nil, err
		}
		result[getSubResourceKey(&csc)] = csc.GetGeneration()
	}
	for _, autolinksRef := range repo.Spec.AutolinksPresetList {
		var autolinksPresetList v1alpha1.AutolinksPreset
		if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: autolinksRef.Name, Namespace: repo.Namespace}, &autolinksPresetList); err != nil {
			log.Error(err, "failed to get AttachedCodeSecurityConfiguration for Repository")
			return nil, err
		}
		result[getSubResourceKey(&autolinksPresetList)] = autolinksPresetList.GetGeneration()
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func getSubResourceKey(obj client.Object) string {
	return fmt.Sprintf("%s/%s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())
}

func buildGitHubOrgsSlice(ctx context.Context, f *Factory, team v1alpha1.Team, refExtractor func(t v1alpha1.Team) []v1alpha1.OrganizationRef) ([]reconciler.GitHub[string], error) {
	log := logPkg.FromContext(ctx)
	var orgs []v1alpha1.Organization
	for _, orgRef := range refExtractor(team) {
		org, err := f.getOrgByRef(ctx, orgRef.Name, team.Namespace)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	var githubOrgs []reconciler.GitHub[string]
	for _, org := range orgs {
		appConfig, err := org.ResolveGitHubAppConfig(f.LegacySecretName)
		if err != nil {
			log.Error(err, "unable to resolve GitHub App config for Organization", "organization", org.GetLogin())
			return nil, err
		}
		ghRepo, err := f.ClientManager.GetGitHubClientAndCheckRateLimit(ctx, org.GetLogin(), *appConfig, teamRateLimitThreshold)
		if err != nil {
			log.Error(err, "unable to get github client for installationId", "organization", org.GetLogin(), "installationId", appConfig.InstallationId)
			return nil, err
		}

		githubOrgs = append(githubOrgs, reconciler.GitHub[string]{
			Client:   ghRepo,
			Resource: org.GetLogin(),
		})
	}

	return githubOrgs, nil
}
