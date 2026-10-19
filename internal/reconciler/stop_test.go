package reconciler

import (
	"context"
	"errors"
	"sync/atomic"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	applyconfiguration "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration"
	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Executor condition handling", func() {
	var (
		ctx        context.Context
		fetchedOrg *githubv1alpha1.Organization
		newExec    func(groups []ParallelReconciliationGroup) *ReconciliationExecutor[*githubv1alpha1.Organization]
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(githubv1alpha1.AddToScheme(scheme)).To(Succeed())
		org := &githubv1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "test-org", Namespace: "default"},
			Spec:       githubv1alpha1.OrganizationSpec{Name: "test-org", GitHubAppInstallationId: new(int64(12345))},
		}
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithTypeConverters(applyconfiguration.NewTypeConverter(scheme), managedfields.NewDeducedTypeConverter()).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()
		fetchedOrg = &githubv1alpha1.Organization{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-org", Namespace: "default"}, fetchedOrg)).To(Succeed())
		newExec = func(groups []ParallelReconciliationGroup) *ReconciliationExecutor[*githubv1alpha1.Organization] {
			return &ReconciliationExecutor[*githubv1alpha1.Organization]{
				Reconciler: &MockReconciler{
					k8s: Kubernetes[*githubv1alpha1.Organization]{
						Client:                        k8sClient,
						Resource:                      fetchedOrg,
						CurrentSubResourceGenerations: map[string]int64{"RulesetPreset/default/p": 2},
					},
					reconciliations: groups,
				},
			}
		}
	})

	returning := func(condition conditions.ConditionType, err error) Reconciliation {
		return Reconciliation{Condition: condition, Function: func(context.Context) error { return err }}
	}
	condition := func(t conditions.ConditionType) *metav1.Condition {
		return meta.FindStatusCondition(fetchedOrg.Status.Conditions, string(t))
	}

	Context("StopReconciliation", func() {
		It("skips the remaining groups and treats the run as successful", func() {
			var laterGroupRan atomic.Bool
			exec := newExec([]ParallelReconciliationGroup{
				{returning(conditions.TypeBaseSettingsSynced, &StopReconciliation{Message: "repository is archived"})},
				{
					{Condition: conditions.TypeRulesetsSynced, Function: func(context.Context) error {
						laterGroupRan.Store(true)
						return nil
					}},
					returning(conditions.TypeActionsConfigurationSynced, nil),
				},
			})

			Expect(exec.Reconcile(ctx)).To(Succeed())
			Expect(laterGroupRan.Load()).To(BeFalse())

			base := condition(conditions.TypeBaseSettingsSynced)
			Expect(base.Status).To(Equal(metav1.ConditionTrue))
			Expect(base.Message).To(ContainSubstring("repository is archived"))

			for _, t := range []conditions.ConditionType{conditions.TypeRulesetsSynced, conditions.TypeActionsConfigurationSynced} {
				skipped := condition(t)
				Expect(skipped).NotTo(BeNil())
				Expect(skipped.Status).To(Equal(metav1.ConditionUnknown))
				Expect(skipped.Reason).To(Equal(conditions.ReasonSkipped))
				Expect(skipped.Message).To(ContainSubstring("repository is archived"))
			}

			ready := condition(conditions.TypeReady)
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			Expect(ready.Reason).To(Equal(conditions.ReasonReconcileCompleted))
			Expect(ready.Message).To(ContainSubstring("repository is archived"))
			Expect(fetchedOrg.Status.ObservedSubResourceGenerations).To(HaveKeyWithValue("RulesetPreset/default/p", int64(2)))
		})

		It("still fails if another reconciliation of the same group failed", func() {
			exec := newExec([]ParallelReconciliationGroup{
				{
					returning(conditions.TypeBaseSettingsSynced, &StopReconciliation{Message: "stopped"}),
					returning(conditions.TypeCustomPropertyDefinitionsSynced, errors.New("boom")),
				},
				{returning(conditions.TypeRulesetsSynced, nil)},
			})

			err := exec.Reconcile(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("boom"))
			Expect(condition(conditions.TypeBaseSettingsSynced).Status).To(Equal(metav1.ConditionTrue))
			Expect(condition(conditions.TypeRulesetsSynced)).To(BeNil())
			Expect(condition(conditions.TypeReady).Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Context("Ready reason", func() {
		It("is UserActionRequired if all failures require user action", func() {
			exec := newExec([]ParallelReconciliationGroup{{
				returning(conditions.TypeRulesetsSynced, &rulesets.ConflictError{Name: "r"}),
				returning(conditions.TypeBaseSettingsSynced, nil),
			}})

			err := exec.Reconcile(ctx)
			Expect(err).To(HaveOccurred())
			Expect(RequiresUserAction(err)).To(BeTrue())
			Expect(condition(conditions.TypeRulesetsSynced).Reason).To(Equal(conditions.ReasonUserActionRequired))
			ready := condition(conditions.TypeReady)
			Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			Expect(ready.Reason).To(Equal(conditions.ReasonUserActionRequired))
		})

		It("is ReconcileFailed if any failure is transient", func() {
			exec := newExec([]ParallelReconciliationGroup{{
				returning(conditions.TypeRulesetsSynced, &rulesets.ConflictError{Name: "r"}),
				returning(conditions.TypeBaseSettingsSynced, errors.New("boom")),
			}})

			Expect(exec.Reconcile(ctx)).NotTo(Succeed())
			Expect(condition(conditions.TypeRulesetsSynced).Reason).To(Equal(conditions.ReasonUserActionRequired))
			Expect(condition(conditions.TypeBaseSettingsSynced).Reason).To(Equal(conditions.ReasonSyncFailed))
			Expect(condition(conditions.TypeReady).Reason).To(Equal(conditions.ReasonReconcileFailed))
		})
	})
})
