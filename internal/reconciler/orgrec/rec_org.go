package orgrec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/internal/mapper"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileGitHubOrg configures the GitHub organization to match the desired state defined in the Kubernetes resource.
func (o *GitHubOrgReconciler) reconcileOrganization(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling organization settings on GitHub")

	ghOrg, err := o.GitHub.Client.GetOrganization(ctx, o.GitHub.Resource)
	if err != nil {
		return err
	}
	if ghOrg == nil {
		return errors.New("received nil organization from GitHub")
	}
	if !mapper.OrgDiffers(o.Kubernetes.Resource, *ghOrg) {
		log.V(1).Info("Organization already up to date on GitHub, skipping update of settings")
		return nil
	}

	wantOrg := mapper.OrgToGithubOrg(o.Kubernetes.Resource)

	if _, err := o.GitHub.Client.EditOrganization(ctx, o.GitHub.Resource, wantOrg); err != nil {
		log.Error(err, "failed to update organization settings on GitHub")
		return err
	}

	log.V(1).Info("Successfully reconciled organization settings on GitHub")
	return nil
}
