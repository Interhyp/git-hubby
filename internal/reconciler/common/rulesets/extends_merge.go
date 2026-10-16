package rulesets

import (
	"strconv"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
)

// MergeKind describes how a RulesetSpec field of a child ruleset is merged onto its resolved parent.
type MergeKind string

const (
	// MergeOwn fields are never inherited.
	MergeOwn MergeKind = "own"
	// MergeInherit fields take the child's value if set (non-empty, non-nil, non-zero), otherwise the parent's.
	MergeInherit MergeKind = "inherit"
	// MergeRecursive objects are merged field by field.
	MergeRecursive MergeKind = "recursive"
	// MergeKeyed lists are merged by key: a child entry replaces the parent entries with the same key in place,
	// other child entries are appended.
	MergeKeyed MergeKind = "keyed"
	// MergeAtomic fields are replaced completely by the child's value if set.
	MergeAtomic MergeKind = "atomic"
)

// MergeRules documents the merge behaviour of every RulesetSpec field by JSON path. It must be kept in sync with
// mergeSpec; a unit test verifies that every field is listed and behaves as declared.
var MergeRules = map[string]MergeKind{
	"name":        MergeOwn,
	"extends":     MergeOwn,
	"target":      MergeInherit,
	"conditions":  MergeAtomic,
	"enforcement": MergeInherit,
	// key: actorType + actorSlug/actorId
	"bypassActors":                                     MergeKeyed,
	"rules":                                            MergeRecursive,
	"rules.creation":                                   MergeInherit,
	"rules.update":                                     MergeInherit,
	"rules.deletion":                                   MergeInherit,
	"rules.requiredLinearHistory":                      MergeInherit,
	"rules.requiredSignatures":                         MergeInherit,
	"rules.nonFastForward":                             MergeInherit,
	"rules.pullRequest":                                MergeRecursive,
	"rules.pullRequest.allowedMergeMethods":            MergeAtomic,
	"rules.pullRequest.dismissStaleReviewsOnPush":      MergeInherit,
	"rules.pullRequest.requireCodeOwnerReviews":        MergeInherit,
	"rules.pullRequest.requireLastPushApproval":        MergeInherit,
	"rules.pullRequest.requiredApprovingReviewCount":   MergeInherit,
	"rules.pullRequest.requiredReviewThreadResolution": MergeInherit,
	// key: reviewer type + slug/id
	"rules.pullRequest.requiredReviewers": MergeKeyed,
	"rules.requiredStatusChecks":          MergeRecursive,
	// key: context + appSlug/integrationId
	"rules.requiredStatusChecks.checks":           MergeKeyed,
	"rules.requiredStatusChecks.strictPolicy":     MergeInherit,
	"rules.commitMessagePattern":                  MergeAtomic,
	"rules.commitAuthorEmailPattern":              MergeAtomic,
	"rules.committerEmailPattern":                 MergeAtomic,
	"rules.branchNamePattern":                     MergeAtomic,
	"rules.tagNamePattern":                        MergeAtomic,
	"rules.copilotReview":                         MergeRecursive,
	"rules.copilotReview.reviewOnPush":            MergeInherit,
	"rules.copilotReview.reviewDraftPullRequests": MergeInherit,
	"rules.workflows":                             MergeRecursive,
	"rules.workflows.doNotEnforceOnCreate":        MergeInherit,
	// key: repositoryName + path
	"rules.workflows.workflows": MergeKeyed,
}

// mergeSpec merges child onto parent according to MergeRules. Both arguments must be owned by the caller (deep
// copies); the result shares memory with them.
func mergeSpec(parent, child v1alpha1.RulesetSpec) v1alpha1.RulesetSpec {
	out := parent
	out.Name = child.Name
	out.Extends = child.Extends
	out.Target = inherit(parent.Target, child.Target)
	out.Conditions = inherit(parent.Conditions, child.Conditions)
	out.Enforcement = inherit(parent.Enforcement, child.Enforcement)
	out.BypassActors = mergeKeyed(parent.BypassActors, child.BypassActors, bypassActorKey)
	out.Rules = mergeRules(parent.Rules, child.Rules)
	return out
}

func mergeRules(parent, child v1alpha1.RulesetRules) v1alpha1.RulesetRules {
	out := parent
	out.Creation = inherit(parent.Creation, child.Creation)
	out.Update = inherit(parent.Update, child.Update)
	out.Deletion = inherit(parent.Deletion, child.Deletion)
	out.RequiredLinearHistory = inherit(parent.RequiredLinearHistory, child.RequiredLinearHistory)
	out.RequiredSignatures = inherit(parent.RequiredSignatures, child.RequiredSignatures)
	out.NonFastForward = inherit(parent.NonFastForward, child.NonFastForward)
	out.PullRequest = mergeObject(parent.PullRequest, child.PullRequest, mergePullRequest)
	out.RequiredStatusChecks = mergeObject(parent.RequiredStatusChecks, child.RequiredStatusChecks, mergeStatusChecks)
	out.CommitMessagePattern = inherit(parent.CommitMessagePattern, child.CommitMessagePattern)
	out.CommitAuthorEmailPattern = inherit(parent.CommitAuthorEmailPattern, child.CommitAuthorEmailPattern)
	out.CommitterEmailPattern = inherit(parent.CommitterEmailPattern, child.CommitterEmailPattern)
	out.BranchNamePattern = inherit(parent.BranchNamePattern, child.BranchNamePattern)
	out.TagNamePattern = inherit(parent.TagNamePattern, child.TagNamePattern)
	out.CopilotReview = mergeObject(parent.CopilotReview, child.CopilotReview, mergeCopilotReview)
	out.Workflows = mergeObject(parent.Workflows, child.Workflows, mergeWorkflows)
	return out
}

func mergePullRequest(parent, child v1alpha1.PullRequestRule) v1alpha1.PullRequestRule {
	out := parent
	if len(child.AllowedMergeMethods) > 0 {
		out.AllowedMergeMethods = child.AllowedMergeMethods
	}
	out.DismissStaleReviewsOnPush = inherit(parent.DismissStaleReviewsOnPush, child.DismissStaleReviewsOnPush)
	out.RequireCodeOwnerReviews = inherit(parent.RequireCodeOwnerReviews, child.RequireCodeOwnerReviews)
	out.RequireLastPushApproval = inherit(parent.RequireLastPushApproval, child.RequireLastPushApproval)
	out.RequiredApprovingReviewCount = inherit(parent.RequiredApprovingReviewCount, child.RequiredApprovingReviewCount)
	out.RequiredReviewThreadResolution = inherit(parent.RequiredReviewThreadResolution, child.RequiredReviewThreadResolution)
	out.RequiredReviewers = mergeKeyed(parent.RequiredReviewers, child.RequiredReviewers, requiredReviewerKey)
	return out
}

func mergeStatusChecks(parent, child v1alpha1.RequiredStatusChecks) v1alpha1.RequiredStatusChecks {
	out := parent
	out.Checks = mergeKeyed(parent.Checks, child.Checks, statusCheckKey)
	out.StrictPolicy = inherit(parent.StrictPolicy, child.StrictPolicy)
	return out
}

func mergeCopilotReview(parent, child v1alpha1.CopilotCodeReviewRule) v1alpha1.CopilotCodeReviewRule {
	out := parent
	out.ReviewOnPush = inherit(parent.ReviewOnPush, child.ReviewOnPush)
	out.ReviewDraftPullRequests = inherit(parent.ReviewDraftPullRequests, child.ReviewDraftPullRequests)
	return out
}

func mergeWorkflows(parent, child v1alpha1.WorkflowsRule) v1alpha1.WorkflowsRule {
	out := parent
	out.DoNotEnforceOnCreate = inherit(parent.DoNotEnforceOnCreate, child.DoNotEnforceOnCreate)
	out.Workflows = mergeKeyed(parent.Workflows, child.Workflows, func(w v1alpha1.RuleWorkflow) string {
		return w.RepositoryName + "/" + w.Path
	})
	return out
}

func bypassActorKey(a v1alpha1.RulesetBypassActor) string {
	return a.ActorType + "/" + slugOrID(a.ActorSlug, a.ActorID)
}

// statusCheckKey identifies a status check by its context and the app providing it, as the same context may be
// required from different apps.
func statusCheckKey(c v1alpha1.StatusCheck) string {
	return c.Context + "/" + slugOrID(c.AppSlug, c.IntegrationID)
}

func requiredReviewerKey(r v1alpha1.RequiredPullRequestReviewer) string {
	return r.Reviewer.Type + "/" + slugOrID(r.Reviewer.Slug, r.Reviewer.ID)
}

func slugOrID(slug *string, id *int64) string {
	switch {
	case slug != nil:
		return "slug:" + *slug
	case id != nil:
		return "id:" + strconv.FormatInt(*id, 10)
	default:
		return ""
	}
}

// inherit returns child if it is set (non-zero), otherwise parent.
func inherit[T comparable](parent, child T) T {
	var zero T
	if child != zero {
		return child
	}
	return parent
}

// mergeObject merges two optional objects with the given merge function.
func mergeObject[T any](parent, child *T, merge func(parent, child T) T) *T {
	if child == nil {
		return parent
	}
	if parent == nil {
		return child
	}
	merged := merge(*parent, *child)
	return &merged
}

// mergeKeyed merges child entries into parent entries by key: the first parent entry with a key also defined by the
// child is replaced in place by the child entry (later parent entries with that key are dropped), child entries with
// new keys are appended in child order. If the child defines a key multiple times, its last entry wins.
func mergeKeyed[T any](parent, child []T, key func(T) string) []T {
	if len(child) == 0 {
		return parent
	}
	childByKey := make(map[string]T, len(child))
	for _, e := range child {
		childByKey[key(e)] = e
	}
	out := make([]T, 0, len(parent)+len(child))
	done := make(map[string]bool, len(child))
	for _, e := range parent {
		k := key(e)
		c, overridden := childByKey[k]
		switch {
		case !overridden:
			out = append(out, e)
		case !done[k]:
			out = append(out, c)
			done[k] = true
		}
	}
	for _, e := range child {
		if k := key(e); !done[k] {
			out = append(out, childByKey[k])
			done[k] = true
		}
	}
	return out
}
