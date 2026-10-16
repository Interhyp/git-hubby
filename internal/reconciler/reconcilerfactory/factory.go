package reconcilerfactory

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/idresolver"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/Interhyp/git-hubby/internal/reconciler/orgrec"
	"github.com/Interhyp/git-hubby/internal/reconciler/reporec"
	"github.com/Interhyp/git-hubby/internal/reconciler/teamrec"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

type Factory struct {
	ClientManager    reconciler.GitHubClientManager
	K8sClient        client.Client
	SpreadingManager reconciler.SpreadManager
	Config           config.Config
}

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

	appConfig, err := orgAppConfig(&org)
	if err != nil {
		logPkg.FromContext(ctx).Error(err, "unable to resolve GitHub App config for Organization")
		return nil, err
	}

	ghClient, err := f.ClientManager.GetClient(ctx, org.GetLogin(), appConfig)
	if err != nil {
		return nil, err
	}

	nameResolverOrg, err := idresolver.New(ctx, ghClient, org.GetLogin())
	if err != nil {
		log.Error(err, "Failed to warm name resolver for Organization", "organization", org.GetLogin())
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
			Features:   f.Config.Features,
			IdResolver: nameResolverOrg,
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

	appConfig, err := orgAppConfig(&org)
	if err != nil {
		log.Error(err, "unable to resolve GitHub App config for Organization", "organization", org.GetLogin())
		return nil, err
	}

	ghClient, err := f.ClientManager.GetClient(ctx, org.GetLogin(), appConfig)
	if err != nil {
		return nil, err
	}

	nameResolverRepo, err := idresolver.New(ctx, ghClient, org.GetLogin())
	if err != nil {
		log.Error(err, "Failed to warm name resolver for Repository", "organization", org.GetLogin())
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
			FinalizeMode: reconciler.RepositoryFinalizerMode(f.Config.RepositoryFinalizerMode),
			Features:     f.Config.Features,
			NameResolver: nameResolverRepo,
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
			MemberSuffix: f.Config.GitHubMemberSuffix,
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
	ctx = logPkg.IntoContext(ctx, logPkg.FromContext(ctx).WithValues("method", "fetchSubResourceGenerationsForOrg"))

	result := make(map[string]int64)

	if err := f.collectRulesetPresetGenerations(ctx, result, org.Namespace, org.Spec.RulesetPresetList, org.Spec.Rulesets); err != nil {
		return nil, err
	}
	for _, cscRef := range org.Spec.CodeSecurityConfigurations {
		if err := f.collectSubResourceGeneration(ctx, result, &v1alpha1.CodeSecurityConfiguration{}, cscRef.Name, org.Namespace); err != nil {
			return nil, err
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func (f *Factory) fetchSubResourceGenerationsForRepo(ctx context.Context, repo v1alpha1.Repository) (map[string]int64, error) {
	ctx = logPkg.IntoContext(ctx, logPkg.FromContext(ctx).WithValues("method", "fetchSubResourceGenerationsForRepo"))

	result := make(map[string]int64)

	if err := f.collectRulesetPresetGenerations(ctx, result, repo.Namespace, repo.Spec.RulesetPresetList, repo.Spec.Rulesets); err != nil {
		return nil, err
	}
	for _, webhookPresetRef := range repo.Spec.WebhookPresetList {
		if err := f.collectSubResourceGeneration(ctx, result, &v1alpha1.WebhookPreset{}, webhookPresetRef.Name, repo.Namespace); err != nil {
			return nil, err
		}
	}
	for _, webhookIgnoreRef := range repo.Spec.WebhookIgnorePresetsList {
		if err := f.collectSubResourceGeneration(ctx, result, &v1alpha1.WebhookIgnorePreset{}, webhookIgnoreRef.Name, repo.Namespace); err != nil {
			return nil, err
		}
	}
	if repo.Spec.AttachedCodeSecurityConfiguration != nil {
		if err := f.collectSubResourceGeneration(ctx, result, &v1alpha1.CodeSecurityConfiguration{}, repo.Spec.AttachedCodeSecurityConfiguration.Name, repo.Namespace); err != nil {
			return nil, err
		}
	}
	for _, autolinksRef := range repo.Spec.AutolinksPresetList {
		if err := f.collectSubResourceGeneration(ctx, result, &v1alpha1.AutolinksPreset{}, autolinksRef.Name, repo.Namespace); err != nil {
			return nil, err
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// collectSubResourceGeneration fetches the referenced sub-resource and records its generation in result.
//
// Sub-resource generations are only used for startup spreading / change detection. A missing sub-resource
// (e.g. a preset that was deleted after its content was moved inline, possibly before the referencing resource
// was updated) must therefore not block the reconciliation: it is skipped here and reported by the responsible
// reconciliation via its status condition instead. Any other error is returned.
func (f *Factory) collectSubResourceGeneration(ctx context.Context, result map[string]int64, obj client.Object, name, namespace string) error {
	log := logPkg.FromContext(ctx)
	kind := fmt.Sprintf("%T", obj)

	if err := f.K8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("referenced sub-resource not found, skipping generation tracking", "kind", kind, "name", name)
			return nil
		}
		log.Error(err, "failed to get referenced sub-resource", "kind", kind, "name", name)
		return err
	}
	result[getSubResourceKey(obj)] = obj.GetGeneration()
	return nil
}

// collectRulesetPresetGenerations records the generations of the referenced RulesetPresets and of all presets their
// extends chains (and those of the inline rulesets) consist of, so a change anywhere in a chain is detected.
func (f *Factory) collectRulesetPresetGenerations(ctx context.Context, result map[string]int64, namespace string,
	refs []corev1.LocalObjectReference, inline []v1alpha1.RulesetSpec) error {
	resolver := rulesets.NewChainResolver(f.K8sClient, namespace)
	collectAncestors := func(extends, self string) error {
		ancestors, err := resolver.Ancestors(ctx, extends, self)
		if err != nil {
			logPkg.FromContext(ctx).Error(err, "failed to get extended RulesetPresets", "extends", extends)
			return err
		}
		for _, preset := range ancestors {
			result[getSubResourceKey(preset)] = preset.GetGeneration()
		}
		return nil
	}
	for _, ref := range refs {
		preset := &v1alpha1.RulesetPreset{}
		if err := f.collectSubResourceGeneration(ctx, result, preset, ref.Name, namespace); err != nil {
			return err
		}
		if err := collectAncestors(preset.Spec.Extends, ref.Name); err != nil {
			return err
		}
	}
	for i := range inline {
		if err := collectAncestors(inline[i].Extends, ""); err != nil {
			return err
		}
	}
	return nil
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
		appConfig, err := orgAppConfig(&org)
		if err != nil {
			log.Error(err, "unable to resolve GitHub App config for Organization", "organization", org.GetLogin())
			return nil, err
		}
		ghRepo, err := f.ClientManager.GetClient(ctx, org.GetLogin(), appConfig)
		if err != nil {
			log.Error(err, "unable to get github client for installationId", "organization", org.GetLogin(), "installationId", appConfig.InstallationID)
			return nil, err
		}

		githubOrgs = append(githubOrgs, reconciler.GitHub[string]{
			Client:   ghRepo,
			Resource: org.GetLogin(),
		})
	}

	return githubOrgs, nil
}

// orgAppConfig extracts the minimal AppConfig needed by the GitHub client manager from an Organization.
// CredentialsSecretName is left empty for legacy orgs (those using the deprecated GitHubAppInstallationId
// field); the client manager will substitute its own legacySecretName in that case.
func orgAppConfig(org *v1alpha1.Organization) (ghclient.AppConfig, error) {
	installationID, err := org.GetGitHubAppInstallationID()
	if err != nil {
		return ghclient.AppConfig{}, err
	}
	return ghclient.AppConfig{
		InstallationID:        installationID,
		CredentialsSecretName: org.GetGitHubAppCredentialsSecretName(),
	}, nil
}
