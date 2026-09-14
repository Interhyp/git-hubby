package reporec

import (
	"context"
	"maps"
	"slices"

	"github.com/ettle/strcase"
	"golang.org/x/text/transform"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileTopics(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository topics on GitHub")

	currentTopics, err := r.GitHub.Client.GetAllTopics(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "Failed to get current topics from GitHub")
		return err
	}
	currentTopics = uniqueKebabCasedSorted(currentTopics)

	desiredTopics := r.uniqueKebabCasedSortedTopics()

	if !slices.Equal(currentTopics, desiredTopics) {
		log.V(1).Info("Updating repository topics on GitHub", "current", currentTopics, "desired", desiredTopics)
		if err := r.GitHub.Client.ReplaceAllTopics(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, desiredTopics); err != nil {
			log.Error(err, "Failed to update topics on GitHub")
			return err
		}
	} else {
		log.V(1).Info("Repository topics on GitHub are already up to date")
	}

	log.V(1).Info("Successfully reconciled repository topics on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) uniqueKebabCasedSortedTopics() []string {
	topics := []string{}
	for _, topic := range r.Kubernetes.Resource.Spec.About.Topics {
		if topic.Name == "" {
			continue
		}
		topics = append(topics, topic.Name)
	}

	return uniqueKebabCasedSorted(topics)
}

func uniqueKebabCasedSorted(input []string) []string {
	m := make(map[string]any)
	transform.Chain()
	for _, in := range input {
		m[strcase.ToKebab(in)] = nil
	}
	out := slices.Collect(maps.Keys(m))
	slices.Sort(out)
	return out
}
