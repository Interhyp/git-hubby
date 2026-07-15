package reconciler

import (
	"context"

	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldOwner is the field manager name used for Server-Side Apply operations.
const FieldOwner = client.FieldOwner("git-hubby")

type GitHubClientManager interface {
	GetGitHubClientAndCheckRateLimit(ctx context.Context, cacheKey string, app ghclient.AppConfig, rateLimitMinimum int) (ghclient.GitHubClient, error)
}

type SpreadManager interface {
	Spread(ctx context.Context, resource spreading.SpreadableResource, currentSubResourceGenerations map[string]int64) error
}

type GitHub[T any] struct {
	Client   ghclient.GitHubClient
	Resource T
}

type GitHubTeamIdentifier struct {
	Organizations ReferencedOrganizations
	Name          string
	// Slug is a pointer so that it can be updated during reconciliation
	Slug *string
}

func (g *GitHubTeamIdentifier) GetSlug() string {
	if g == nil || g.Slug == nil {
		return ""
	}
	return *g.Slug
}

type ReferencedOrganizations struct {
	Current  []GitHub[string]
	Previous []GitHub[string]
}

type Kubernetes[T ReconcilableResource] struct {
	Client   client.Client
	Resource T

	// CurrentSubResourceGenerations is a map of sub-resource names to their current generation.
	// Keys are in the format "<kind>/<namespace/<name>".
	CurrentSubResourceGenerations map[string]int64
}

type Reconciliation struct {
	Function  func(context.Context) error
	Condition conditions.ConditionType
}

// ReconcilableResource interface that all reconcilable resources must implement
type ReconcilableResource interface {
	client.Object
	GetConditions() *[]metav1.Condition
	GetTypeRepresentation() string
	SetObservedSubResourceGeneration(new map[string]int64)
}

type ParallelReconciliationGroup []Reconciliation

type Reconciler[T ReconcilableResource] interface {
	// GetAdditionalLogFields returns a slice of keys and values that are added to the logs for all logs produced during the reconciliation. Every odd entry is interpreted as a log field key followed by the
	GetAdditionalLogFields() []any
	// K8s encapsulates access to the K8s resource of type T which is the reconciliation target
	K8s() Kubernetes[T]
	// GetAdditionalLabels returns additional, resource type specific labels to be added to the K8s resource during reconciliation
	GetAdditionalLabels() labels.Set
	// RequiredReconciliations returns the list of reconciliations to be performed for the resource
	RequiredReconciliations() []ParallelReconciliationGroup
	// FinalizerName of the finalizer which is managed by this reconciler
	FinalizerName() string
	// ReconcileDeletion performs all operations required before the K8s resource of type T can be deleted.
	// Any non-nil error will prevent deletion of the resource by keeping the finalizer in place.
	ReconcileDeletion(ctx context.Context) error
	// BuildMetadataApplyConfig builds an SSA apply configuration containing only metadata (labels, annotations, finalizers).
	BuildMetadataApplyConfig(desiredLabels map[string]string, desiredAnnotations map[string]string, finalizers []string) runtime.ApplyConfiguration
	// BuildStatusApplyConfig builds an SSA apply configuration containing the complete status.
	// The reconciler reads all status fields (conditions, observedSubResourceGenerations, and
	// resource-specific fields like ID, webhooks, slug, previousOrganizationRefs) directly
	// from the in-memory K8s resource. This ensures a single Status().Apply() call writes
	// all status fields atomically without overwriting fields set by reconciliation tasks.
	BuildStatusApplyConfig() runtime.ApplyConfiguration
}

type RepositoryFinalizerMode string

const (
	Ignore  RepositoryFinalizerMode = "ignore"
	Archive RepositoryFinalizerMode = "archive"
	Delete  RepositoryFinalizerMode = "delete"
)
