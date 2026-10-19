package reconciler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/google/go-github/v93/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func unprocessable(message string, details ...github.Error) *github.ErrorResponse {
	req, _ := http.NewRequest(http.MethodPatch, "https://api.github.com/repos/org/repo", nil)
	return &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusUnprocessableEntity, Request: req},
		Message:  message,
		Errors:   details,
	}
}

var _ = Describe("RequiresUserAction", func() {
	unresolved := func(reason rulesets.UnresolvedReason) error {
		return &rulesets.UnresolvedError{Ruleset: "r", Reason: reason}
	}
	serverError := &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusBadGateway}, Message: "Bad Gateway"}

	DescribeTable("classifies errors",
		func(err error, expected bool) {
			Expect(RequiresUserAction(err)).To(Equal(expected))
		},
		Entry("nil", nil, false),
		Entry("generic error", errors.New("boom"), false),
		Entry("missing parent", unresolved(rulesets.ReasonParentNotFound), true),
		Entry("cycle", unresolved(rulesets.ReasonExtendsCycle), true),
		Entry("transient read error", unresolved(rulesets.ReasonParentUnavailable), false),
		Entry("transient read error wrapping a marked error",
			&rulesets.UnresolvedError{Ruleset: "r", Reason: rulesets.ReasonParentUnavailable, Err: &rulesets.ConflictError{}}, false),
		Entry("conflict", &rulesets.ConflictError{Name: "r"}, true),
		Entry("GitHub 422", unprocessable("Validation Failed"), true),
		Entry("GitHub 502", serverError, false),
		Entry("GitHub error without response", &github.ErrorResponse{}, false),
		Entry("wrapped and joined marked errors",
			fmt.Errorf("sync: %w", errors.Join(unresolved(rulesets.ReasonParentNotFound), errors.Join(unprocessable("Validation Failed")))), true),
		Entry("marked error joined with a transient read error",
			errors.Join(unresolved(rulesets.ReasonParentNotFound), unresolved(rulesets.ReasonParentUnavailable)), false),
		Entry("marked error joined with another error",
			errors.Join(unresolved(rulesets.ReasonParentNotFound), errors.New("webhook failed")), false),
		Entry("marked error joined with a GitHub 502", errors.Join(&rulesets.ConflictError{}, serverError), false),
		Entry("empty join wrapper", fmt.Errorf("x: %w", errors.Join()), false),
		Entry("readable error wrapper", withReadableMessage(unprocessable("Validation Failed")), true),
	)
})

var _ = Describe("AwaitsGitHubChange", func() {
	DescribeTable("classifies errors",
		func(err error, expected bool) {
			Expect(AwaitsGitHubChange(err)).To(Equal(expected))
		},
		Entry("nil", nil, false),
		Entry("generic error", errors.New("boom"), false),
		Entry("conflict (fixable in Kubernetes only)", &rulesets.ConflictError{Name: "r"}, false),
		Entry("GitHub 422", unprocessable("Validation Failed"), true),
		Entry("GitHub 502", &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusBadGateway}}, false),
		Entry("marked error", gitHubStateErr{awaits: true}, true),
		Entry("marked error returning false", gitHubStateErr{awaits: false}, false),
		Entry("any joined error awaiting a GitHub change",
			fmt.Errorf("sync: %w", errors.Join(&rulesets.ConflictError{Name: "r"}, gitHubStateErr{awaits: true})), true),
		Entry("readable error wrapper", withReadableMessage(unprocessable("Validation Failed")), true),
	)
})

type gitHubStateErr struct{ awaits bool }

func (e gitHubStateErr) Error() string            { return "github state" }
func (e gitHubStateErr) UserActionRequired() bool { return true }
func (e gitHubStateErr) AwaitsGitHubChange() bool { return e.awaits }

var _ = Describe("ErrorMessage", func() {
	It("returns an empty message for nil", func() {
		Expect(ErrorMessage(nil)).To(BeEmpty())
	})

	It("keeps messages of other errors", func() {
		Expect(ErrorMessage(errors.New("boom"))).To(Equal("boom"))
		serverError := &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusBadGateway}, Message: "Bad Gateway"}
		Expect(ErrorMessage(serverError)).To(Equal(serverError.Error()))
	})

	It("uses GitHub's response message and validation errors for 422 responses", func() {
		ghErr := unprocessable("Validation Failed",
			github.Error{Resource: "Repository", Field: "default_branch", Message: "Cannot update default branch for an empty repository"},
			github.Error{Resource: "Repository", Field: "name", Code: "invalid"},
		)
		err := fmt.Errorf("failed to edit repository: %w", ghErr)
		Expect(ErrorMessage(err)).To(Equal(
			"failed to edit repository: Validation Failed: default_branch: Cannot update default branch for an empty repository; name: invalid"))
	})

	It("replaces all 422 responses in joined errors", func() {
		err := errors.Join(unprocessable("Validation Failed"), errors.New("boom"), unprocessable("Invalid request"))
		Expect(ErrorMessage(err)).To(Equal("Validation Failed\nboom\nInvalid request"))
	})

	It("is used by the readable wrapper which keeps the error tree", func() {
		ghErr := unprocessable("Validation Failed")
		err := withReadableMessage(fmt.Errorf("x: %w", ghErr))
		Expect(err.Error()).To(Equal("x: Validation Failed"))
		Expect(errors.Is(err, ghErr)).To(BeTrue())
	})

	It("does not wrap errors without 422 responses", func() {
		err := errors.New("boom")
		Expect(withReadableMessage(err)).To(BeIdenticalTo(err))
		Expect(withReadableMessage(nil)).To(Succeed())
	})
})

var _ = Describe("conditionFromResult", func() {
	finishedAt := time.Now()

	It("sets SyncFailed for transient errors", func() {
		condition := conditionFromResult(reconciliationResult{
			Condition: conditions.TypeWebhooksSynced, Error: errors.New("boom"), FinishedAt: finishedAt,
		})
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal(conditions.ReasonSyncFailed))
		Expect(condition.Message).To(ContainSubstring("boom"))
	})

	It("sets UserActionRequired with GitHub's message for 422 responses", func() {
		condition := conditionFromResult(reconciliationResult{
			Condition:  conditions.TypeBaseSettingsSynced,
			Error:      fmt.Errorf("update: %w", unprocessable("Validation Failed", github.Error{Field: "default_branch", Message: "Cannot update"})),
			FinishedAt: finishedAt,
		})
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal(conditions.ReasonUserActionRequired))
		Expect(condition.Message).To(ContainSubstring("update: Validation Failed: default_branch: Cannot update"))
		Expect(condition.Message).NotTo(ContainSubstring("api.github.com"))
	})

})
