package reconciler

import (
	"context"
	"errors"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Interhyp/git-hubby/internal/conditions"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	applyconfiguration "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration"
	ac "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type MockReconciler struct {
	k8s             Kubernetes[*githubv1alpha1.Organization]
	reconciliations []ParallelReconciliationGroup
}

func (m MockReconciler) K8s() Kubernetes[*githubv1alpha1.Organization] {
	return m.k8s
}

func (m MockReconciler) GetAdditionalLabels() labels.Set {
	return labels.Set{
		"test-key": m.k8s.Resource.Spec.Name,
	}
}
func (m MockReconciler) GetAdditionalLogFields() []any {
	return nil
}

func (m MockReconciler) RequiredReconciliations() []ParallelReconciliationGroup {
	if m.reconciliations != nil {
		return m.reconciliations
	}
	return []ParallelReconciliationGroup{}
}

func (m MockReconciler) FinalizerName() string {
	return "test-finalizer"
}

func (m MockReconciler) ReconcileDeletion(ctx context.Context) error {
	return nil
}

func (m MockReconciler) BuildMetadataApplyConfig(lbls map[string]string, annotations map[string]string, finalizers []string) runtime.ApplyConfiguration {
	cfg := ac.Organization(m.k8s.Resource.Name, m.k8s.Resource.Namespace)
	if lbls != nil {
		cfg.WithLabels(lbls)
	}
	if annotations != nil {
		cfg.WithAnnotations(annotations)
	}
	if finalizers != nil {
		cfg.WithFinalizers(finalizers...)
	}
	return cfg
}

func (m MockReconciler) BuildStatusApplyConfig() runtime.ApplyConfiguration {
	conds := *m.k8s.Resource.GetConditions()
	status := ac.OrganizationStatus().
		WithConditions(ConditionsToApplyConfigs(conds)...)
	if gens := m.k8s.Resource.Status.ObservedSubResourceGenerations; gens != nil {
		status.WithObservedSubResourceGenerations(gens)
	}
	return ac.Organization(m.k8s.Resource.Name, m.k8s.Resource.Namespace).WithStatus(status)
}

var _ = Describe("OrganizationReconciler Labels", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(githubv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	Context("When adding labels to organization", func() {
		It("Should add default labels to organization with no labels", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels).NotTo(BeNil())
			Expect(fetchedOrg.Labels).To(HaveKey("app.kubernetes.io/managed-by"))
			Expect(fetchedOrg.Labels["app.kubernetes.io/managed-by"]).To(Equal("git-hubby"))
			Expect(fetchedOrg.Labels).To(HaveKey("test-key"))
			Expect(fetchedOrg.Labels["test-key"]).To(Equal("test-org"))
		})

		It("Should preserve existing labels and add missing defaults", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
					Labels: map[string]string{
						"existing-label": "existing-value",
					},
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels).To(HaveKey("existing-label"))
			Expect(fetchedOrg.Labels["existing-label"]).To(Equal("existing-value"))
			Expect(fetchedOrg.Labels).To(HaveKey("app.kubernetes.io/managed-by"))
			Expect(fetchedOrg.Labels["app.kubernetes.io/managed-by"]).To(Equal("git-hubby"))
			Expect(fetchedOrg.Labels).To(HaveKey("test-key"))
			Expect(fetchedOrg.Labels["test-key"]).To(Equal("test-org"))
		})

		It("Should override existing enforced label values", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "custom-value",
					},
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels["app.kubernetes.io/managed-by"]).To(Equal("git-hubby"))
		})

		It("Should set organization label to the spec name", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "my-github-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels["test-key"]).To(Equal("my-github-org"))
		})

		It("Should override already set organization label", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
					Labels: map[string]string{
						"test-key": "original-org",
					},
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "new-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels["test-key"]).To(Equal("new-org"))
		})

		It("Should preserve all existing labels", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
					Labels: map[string]string{
						"label1": "value1",
						"label2": "value2",
						"label3": "value3",
					},
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels).To(HaveKey("label1"))
			Expect(fetchedOrg.Labels).To(HaveKey("label2"))
			Expect(fetchedOrg.Labels).To(HaveKey("label3"))
			Expect(fetchedOrg.Labels["label1"]).To(Equal("value1"))
			Expect(fetchedOrg.Labels["label2"]).To(Equal("value2"))
			Expect(fetchedOrg.Labels["label3"]).To(Equal("value3"))
		})

		It("Should handle organization name with special characters", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-org-with-dashes",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "my-org-with-dashes",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "my-org-with-dashes", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "my-org-with-dashes", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			Expect(fetchedOrg.Labels["test-key"]).To(Equal("my-org-with-dashes"))
		})

		It("Should apply labels idempotently", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			// First apply
			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			// Verify labels after first apply using a separate variable to avoid
			// overwriting the reconciler's resource pointer (fake client SSA replaces
			// the stored object, which would clear Spec fields on re-fetch)
			var verifyOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &verifyOrg)).To(Succeed())
			firstLabels := maps.Clone(verifyOrg.Labels)

			// Second apply (reuse same reconciler — Spec.Name is still "test-org" on the resource pointer)
			err = rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &verifyOrg)).To(Succeed())
			secondLabels := verifyOrg.Labels

			// Both should be identical
			Expect(secondLabels).To(Equal(firstLabels))
		})

		It("Should handle labels with multiple enforced labels", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
					Labels: map[string]string{
						"custom-key": "custom-value",
					},
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
				},
			}

			err := rec.applyMetadata(ctx, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			// Verify all default labels are applied
			Expect(fetchedOrg.Labels).To(HaveKey("app.kubernetes.io/managed-by"))
			Expect(fetchedOrg.Labels).To(HaveKey("test-key"))
			Expect(fetchedOrg.Labels).To(HaveKey("custom-key"))
		})
	})

	Context("Parallel Reconciliation Execution", func() {
		It("Should execute reconciliations in a single group concurrently", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
				Status: githubv1alpha1.OrganizationStatus{
					Conditions: []metav1.Condition{},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			// Track execution order and concurrency
			var execCount atomic.Int32
			maxConcurrent := atomic.Int32{}

			makeReconciliation := func(conditionType conditions.ConditionType, delay time.Duration) Reconciliation {
				return Reconciliation{
					Condition: conditionType,
					Function: func(ctx context.Context) error {
						current := execCount.Add(1)
						if current > maxConcurrent.Load() {
							maxConcurrent.Store(current)
						}
						time.Sleep(delay)
						execCount.Add(-1)
						return nil
					},
				}
			}

			reconciliations := []ParallelReconciliationGroup{
				{
					makeReconciliation(conditions.TypeBaseSettingsSynced, 50*time.Millisecond),
					makeReconciliation(conditions.TypeRulesetsSynced, 50*time.Millisecond),
					makeReconciliation(conditions.TypeCustomPropertyDefinitionsSynced, 50*time.Millisecond),
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			start := time.Now()
			_, err := rec.runReconciliations(ctx)
			duration := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			// If run serially, would take 150ms+. Parallel should be ~50ms+
			Expect(duration).To(BeNumerically("<", 100*time.Millisecond))
			// All 3 should have run concurrently
			Expect(maxConcurrent.Load()).To(Equal(int32(3)))
		})

		It("Should execute multiple groups sequentially", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
				Status: githubv1alpha1.OrganizationStatus{
					Conditions: []metav1.Condition{},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			// Track execution order
			var executionOrder []string
			var mu sync.Mutex

			makeReconciliation := func(name string, conditionType conditions.ConditionType) Reconciliation {
				return Reconciliation{
					Condition: conditionType,
					Function: func(ctx context.Context) error {
						mu.Lock()
						executionOrder = append(executionOrder, name)
						mu.Unlock()
						time.Sleep(10 * time.Millisecond)
						return nil
					},
				}
			}

			reconciliations := []ParallelReconciliationGroup{
				{
					makeReconciliation("group1-rec1", conditions.TypeBaseSettingsSynced),
					makeReconciliation("group1-rec2", conditions.TypeRulesetsSynced),
				},
				{
					makeReconciliation("group2-rec1", conditions.TypeCustomPropertyDefinitionsSynced),
					makeReconciliation("group2-rec2", conditions.TypeActionsConfigurationSynced),
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			_, err := rec.runReconciliations(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify group 1 executed before group 2
			Expect(executionOrder).To(HaveLen(4))
			group1Complete := false
			for _, name := range executionOrder {
				if name == "group2-rec1" || name == "group2-rec2" {
					Expect(group1Complete).To(BeTrue(), "Group 2 should not start before Group 1 completes")
				}
				if (name == "group1-rec1" || name == "group1-rec2") &&
					executionOrder[len(executionOrder)-1] != "group1-rec1" &&
					executionOrder[len(executionOrder)-1] != "group1-rec2" {
					group1Complete = true
				}
			}
		})

		It("Should collect errors from all reconciliations without failing fast", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
				Status: githubv1alpha1.OrganizationStatus{
					Conditions: []metav1.Condition{},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			// Track which reconciliations executed
			var executed []string
			var mu sync.Mutex

			makeReconciliation := func(name string, conditionType conditions.ConditionType, shouldFail bool) Reconciliation {
				return Reconciliation{
					Condition: conditionType,
					Function: func(ctx context.Context) error {
						mu.Lock()
						executed = append(executed, name)
						mu.Unlock()
						if shouldFail {
							return errors.New(name + " failed")
						}
						return nil
					},
				}
			}

			reconciliations := []ParallelReconciliationGroup{
				{
					makeReconciliation("rec1", conditions.TypeBaseSettingsSynced, true),
					makeReconciliation("rec2", conditions.TypeRulesetsSynced, false),
					makeReconciliation("rec3", conditions.TypeCustomPropertyDefinitionsSynced, true),
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			_, err := rec.runReconciliations(ctx)

			// Should return error with all failures
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rec1 failed"))
			Expect(err.Error()).To(ContainSubstring("rec3 failed"))

			// All reconciliations should have executed
			Expect(executed).To(HaveLen(3))
			Expect(executed).To(ContainElements("rec1", "rec2", "rec3"))
		})

		It("Should stop after group has at least on error", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
				Status: githubv1alpha1.OrganizationStatus{
					Conditions: []metav1.Condition{},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			reconciliations := []ParallelReconciliationGroup{
				{
					{
						Condition: conditions.TypeBaseSettingsSynced,
						Function: func(ctx context.Context) error {
							return errors.New("group1 error")
						},
					},
				},
				{
					{
						Condition: conditions.TypeRulesetsSynced,
						Function: func(ctx context.Context) error {
							return errors.New("group2 error")
						},
					},
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			_, err := rec.runReconciliations(ctx)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("group1 error"))
		})

		It("Should handle empty reconciliation groups", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
				Status: githubv1alpha1.OrganizationStatus{
					Conditions: []metav1.Condition{},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			reconciliations := []ParallelReconciliationGroup{}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			_, err := rec.runReconciliations(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should execute groups in order even with different completion times", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
				Status: githubv1alpha1.OrganizationStatus{
					Conditions: []metav1.Condition{},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				WithStatusSubresource(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			var group2Started atomic.Bool
			var group1Completed atomic.Bool

			reconciliations := []ParallelReconciliationGroup{
				{
					{
						Condition: conditions.TypeBaseSettingsSynced,
						Function: func(ctx context.Context) error {
							// Group 1 takes longer
							time.Sleep(50 * time.Millisecond)
							group1Completed.Store(true)
							return nil
						},
					},
				},
				{
					{
						Condition: conditions.TypeRulesetsSynced,
						Function: func(ctx context.Context) error {
							// Group 2 is fast but should wait for group 1
							group2Started.Store(true)
							Expect(group1Completed.Load()).To(BeTrue(), "Group 2 should not start before Group 1 completes")
							return nil
						},
					},
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			_, err := rec.runReconciliations(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(group2Started.Load()).To(BeTrue())
		})
	})

	Context("When final status apply fails", func() {
		It("Should return the status apply error even when reconciliations succeed", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			// Build client WITHOUT WithStatusSubresource to make Status().Apply() fail
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			reconciled := false
			reconciliations := []ParallelReconciliationGroup{
				{
					{
						Condition: conditions.TypeBaseSettingsSynced,
						Function: func(ctx context.Context) error {
							reconciled = true
							return nil
						},
					},
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			err := rec.Reconcile(ctx)
			Expect(err).To(HaveOccurred())
			// Reconciliation tasks should have completed before the status write failure
			Expect(reconciled).To(BeTrue())
		})

		It("Should still update in-memory conditions even when status apply fails", func() {
			org := &githubv1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: githubv1alpha1.OrganizationSpec{
					Name:                    "test-org",
					GitHubAppInstallationId: 12345,
				},
			}

			// Build client WITHOUT WithStatusSubresource to make Status().Apply() fail
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
				WithObjects(org).
				Build()

			var fetchedOrg githubv1alpha1.Organization
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, &fetchedOrg)).To(Succeed())

			reconciliations := []ParallelReconciliationGroup{
				{
					{
						Condition: conditions.TypeBaseSettingsSynced,
						Function: func(ctx context.Context) error {
							return nil
						},
					},
				},
			}

			rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:   k8sClient,
						Resource: &fetchedOrg,
					},
					reconciliations: reconciliations,
				},
			}

			_ = rec.Reconcile(ctx)
			// In-memory conditions should be set even though status apply failed
			conds := fetchedOrg.Status.Conditions
			Expect(conds).NotTo(BeEmpty())

			var baseSettingsCondition *metav1.Condition
			for i := range conds {
				if conds[i].Type == string(conditions.TypeBaseSettingsSynced) {
					baseSettingsCondition = &conds[i]
					break
				}
			}
			Expect(baseSettingsCondition).NotTo(BeNil())
			Expect(baseSettingsCondition.Status).To(Equal(metav1.ConditionTrue))
		})
	})

})
