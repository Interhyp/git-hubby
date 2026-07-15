package mapper

import (
	"fmt"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v89/github"
)

// RulesetPresetToGithubRuleset converts a RulesetPreset to a GitHub RepositoryRuleset.
// reconciler.ResolveNamesToIDsInRuleset must have been called before.
func RulesetPresetToGithubRuleset(preset githubv1alpha1.RulesetPreset) (*github.RepositoryRuleset, error) {
	if preset.Spec.Name == "" {
		return nil, fmt.Errorf("ruleset name cannot be empty")
	}

	ruleset := &github.RepositoryRuleset{
		Name:        preset.Spec.Name,
		Enforcement: github.RulesetEnforcement(preset.Spec.Enforcement),
		Target:      new(github.RulesetTarget(preset.Spec.Target)),
	}

	ruleset.Conditions = mapConditions(preset.Spec.Conditions)

	// Map bypass actors
	if len(preset.Spec.BypassActors) > 0 {
		bypassActors := make([]*github.BypassActor, len(preset.Spec.BypassActors))
		for i, actor := range preset.Spec.BypassActors {
			actorType := github.BypassActorType(actor.ActorType)
			bypassActors[i] = &github.BypassActor{
				ActorID:   actor.ActorID,
				ActorType: &actorType,
			}
			if actor.BypassMode != "" {
				bypassMode := github.BypassMode(actor.BypassMode)
				bypassActors[i].BypassMode = &bypassMode
			}
		}
		ruleset.BypassActors = bypassActors
	}

	// Map rules
	mapRules(&preset.Spec.Rules, ruleset)

	return ruleset, nil
}

// mapConditions maps the ruleset conditions (which refs to target)
func mapConditions(k8sConditions *githubv1alpha1.RulesetConditions) *github.RepositoryRulesetConditions {
	if k8sConditions == nil {
		return nil
	}

	conditions := &github.RepositoryRulesetConditions{}
	if k8sConditions.RefName != nil {
		conditions.RefName = &github.RepositoryRulesetRefConditionParameters{
			Include: append(make([]string, 0), k8sConditions.RefName.Include...),
			Exclude: append(make([]string, 0), k8sConditions.RefName.Exclude...),
		}
	}
	if k8sConditions.RepositoryName != nil {
		conditions.RepositoryName = &github.RepositoryRulesetRepositoryNamesConditionParameters{
			Include:   append(make([]string, 0), k8sConditions.RepositoryName.Include...),
			Exclude:   append(make([]string, 0), k8sConditions.RepositoryName.Exclude...),
			Protected: k8sConditions.RepositoryName.Protected,
		}

	} else if k8sConditions.RepositoryProperty != nil {
		conditions.RepositoryProperty = &github.RepositoryRulesetRepositoryPropertyConditionParameters{
			Include: append(make([]*github.RepositoryRulesetRepositoryPropertyTargetParameters, 0), mapRepositoryPropertyTargets(k8sConditions.RepositoryProperty.Include)...),
			Exclude: append(make([]*github.RepositoryRulesetRepositoryPropertyTargetParameters, 0), mapRepositoryPropertyTargets(k8sConditions.RepositoryProperty.Exclude)...),
		}
	}

	return conditions
}

func mapRepositoryPropertyTargets(k8sTargets []githubv1alpha1.RepositoryPropertyTarget) []*github.RepositoryRulesetRepositoryPropertyTargetParameters {
	result := make([]*github.RepositoryRulesetRepositoryPropertyTargetParameters, len(k8sTargets))
	for i, k8sTarget := range k8sTargets {
		result[i] = &github.RepositoryRulesetRepositoryPropertyTargetParameters{
			Name:           k8sTarget.Name,
			PropertyValues: append(make([]string, 0), k8sTarget.PropertyValues...),
			Source:         k8sTarget.Source,
		}
	}
	return result
}

// mapRules maps the rules specification
func mapRules(rules *githubv1alpha1.RulesetRules, ruleset *github.RepositoryRuleset) {
	githubRules := &github.RepositoryRulesetRules{}

	// Creation rule
	if utils.WithDefault(rules.Creation, false) {
		githubRules.Creation = &github.EmptyRuleParameters{}
	}

	// Update rule
	if utils.WithDefault(rules.Update, false) {
		githubRules.Update = &github.UpdateRuleParameters{
			UpdateAllowsFetchAndMerge: true,
		}
	}

	// Deletion rule
	if utils.WithDefault(rules.Deletion, false) {
		githubRules.Deletion = &github.EmptyRuleParameters{}
	}

	// Required linear history
	if utils.WithDefault(rules.RequiredLinearHistory, false) {
		githubRules.RequiredLinearHistory = &github.EmptyRuleParameters{}
	}

	// Non fast forward
	if utils.WithDefault(rules.NonFastForward, false) {
		githubRules.NonFastForward = &github.EmptyRuleParameters{}
	}

	if utils.WithDefault(rules.RequiredSignatures, false) {
		githubRules.RequiredSignatures = &github.EmptyRuleParameters{}
	}

	// Pull request rule
	if rules.PullRequest != nil {
		githubRules.PullRequest = &github.PullRequestRuleParameters{
			AllowedMergeMethods:            mapMergeMethods(rules.PullRequest.AllowedMergeMethods),
			DismissStaleReviewsOnPush:      utils.WithDefault(rules.PullRequest.DismissStaleReviewsOnPush, false),
			RequireCodeOwnerReview:         utils.WithDefault(rules.PullRequest.RequireCodeOwnerReviews, false),
			RequireLastPushApproval:        utils.WithDefault(rules.PullRequest.RequireLastPushApproval, false),
			RequiredApprovingReviewCount:   rules.PullRequest.RequiredApprovingReviewCount,
			RequiredReviewThreadResolution: utils.WithDefault(rules.PullRequest.RequiredReviewThreadResolution, false),
		}
	}

	// Required status checks rule
	if rules.RequiredStatusChecks != nil {
		statusChecks := make([]*github.RuleStatusCheck, len(rules.RequiredStatusChecks.Checks))
		for i, check := range rules.RequiredStatusChecks.Checks {
			statusCheck := &github.RuleStatusCheck{
				Context: check.Context,
			}
			if check.IntegrationID != nil {
				statusCheck.IntegrationID = check.IntegrationID
			}
			statusChecks[i] = statusCheck
		}

		githubRules.RequiredStatusChecks = &github.RequiredStatusChecksRuleParameters{
			RequiredStatusChecks:             statusChecks,
			StrictRequiredStatusChecksPolicy: utils.WithDefault(rules.RequiredStatusChecks.StrictPolicy, false),
		}
	}

	// Copilot code review rule
	if rules.CopilotReview != nil {
		githubRules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{
			ReviewOnPush:            utils.WithDefault(rules.CopilotReview.ReviewOnPush, true),
			ReviewDraftPullRequests: utils.WithDefault(rules.CopilotReview.ReviewDraftPullRequests, true),
		}
	}

	// Workflows rule (org-level only — requires resolved repository IDs)
	if rules.Workflows != nil {
		workflows := make([]*github.RuleWorkflow, len(rules.Workflows.Workflows))
		for i, wf := range rules.Workflows.Workflows {
			workflows[i] = &github.RuleWorkflow{
				Path:         wf.Path,
				RepositoryID: wf.ResolvedRepositoryID,
				Ref:          wf.Ref,
			}
		}
		githubRules.Workflows = &github.WorkflowsRuleParameters{
			DoNotEnforceOnCreate: rules.Workflows.DoNotEnforceOnCreate,
			Workflows:            workflows,
		}
	}

	// Pattern rules
	mapPatternRules(rules, githubRules)

	ruleset.Rules = githubRules
}

func mapMergeMethods(mergeMethods []string) []github.PullRequestMergeMethod {
	githubMergeMethods := make([]github.PullRequestMergeMethod, len(mergeMethods))
	for i, method := range mergeMethods {
		switch method {
		case "rebase":
			githubMergeMethods[i] = github.PullRequestMergeMethodRebase
		case "squash":
			githubMergeMethods[i] = github.PullRequestMergeMethodSquash
		case "merge":
			githubMergeMethods[i] = github.PullRequestMergeMethodMerge
		default:
			githubMergeMethods[i] = github.PullRequestMergeMethodMerge
		}
	}
	if len(githubMergeMethods) == 0 {
		githubMergeMethods = []github.PullRequestMergeMethod{github.PullRequestMergeMethodMerge}
	}
	return githubMergeMethods
}

// mapPatternRules maps pattern-based rules
func mapPatternRules(rules *githubv1alpha1.RulesetRules, githubRules *github.RepositoryRulesetRules) {
	if rules.CommitMessagePattern != nil {
		githubRules.CommitMessagePattern = &github.PatternRuleParameters{
			Pattern:  rules.CommitMessagePattern.Pattern,
			Operator: github.PatternRuleOperator(rules.CommitMessagePattern.Operator),
			Negate:   rules.CommitMessagePattern.Negate,
		}
	}

	if rules.CommitAuthorEmailPattern != nil {
		githubRules.CommitAuthorEmailPattern = &github.PatternRuleParameters{
			Pattern:  rules.CommitAuthorEmailPattern.Pattern,
			Operator: github.PatternRuleOperator(rules.CommitAuthorEmailPattern.Operator),
			Negate:   rules.CommitAuthorEmailPattern.Negate,
		}
	}

	if rules.CommitterEmailPattern != nil {
		githubRules.CommitterEmailPattern = &github.PatternRuleParameters{
			Pattern:  rules.CommitterEmailPattern.Pattern,
			Operator: github.PatternRuleOperator(rules.CommitterEmailPattern.Operator),
			Negate:   rules.CommitterEmailPattern.Negate,
		}
	}

	if rules.BranchNamePattern != nil {
		githubRules.BranchNamePattern = &github.PatternRuleParameters{
			Pattern:  rules.BranchNamePattern.Pattern,
			Operator: github.PatternRuleOperator(rules.BranchNamePattern.Operator),
			Negate:   rules.BranchNamePattern.Negate,
		}
	}

	if rules.TagNamePattern != nil {
		githubRules.TagNamePattern = &github.PatternRuleParameters{
			Pattern:  rules.TagNamePattern.Pattern,
			Operator: github.PatternRuleOperator(rules.TagNamePattern.Operator),
			Negate:   rules.TagNamePattern.Negate,
		}
	}
}

// RulesetsDiffer compares a RulesetPreset with a GitHub RepositoryRuleset to determine if they differ
func RulesetsDiffer(preset githubv1alpha1.RulesetPreset, githubRuleset github.RepositoryRuleset) bool {
	if preset.Spec.Name != githubRuleset.Name {
		return true
	}

	if preset.Spec.Target != string(*utils.WithDefaultAsPtr(githubRuleset.Target, github.RulesetTargetBranch)) {
		return true
	}

	if string(preset.Spec.Enforcement) != string(githubRuleset.Enforcement) {
		return true
	}

	if bypassActorsDiffer(preset.Spec.BypassActors, githubRuleset.BypassActors) {
		return true
	}

	if conditionsDiffer(preset.Spec.Conditions, githubRuleset.Conditions) {
		return true
	}

	// Compare rules
	if rulesetRulesDiffer(preset.Spec.Rules, githubRuleset.Rules) {
		return true
	}

	return false
}

// bypassActorsDiffer compares bypass actors between preset and GitHub ruleset
func bypassActorsDiffer(presetActors []githubv1alpha1.RulesetBypassActor, githubActors []*github.BypassActor) bool {
	if len(presetActors) != len(githubActors) {
		return true
	}

	presetActorMap := make(map[string]githubv1alpha1.RulesetBypassActor)
	keyFunc := func(actorType string, actorID *int64) string {
		return fmt.Sprintf("%s/%d", actorType, utils.WithDefault(actorID, 0))
	}
	for _, actor := range presetActors {
		key := keyFunc(actor.ActorType, actor.ActorID)
		presetActorMap[key] = actor
	}

	githubActorMap := make(map[string]*github.BypassActor)
	for _, actor := range githubActors {
		actorType := "unknown"
		if actor.ActorType != nil {
			actorType = string(*actor.ActorType)
		}
		key := keyFunc(actorType, actor.ActorID)
		githubActorMap[key] = actor
	}

	for actoryKey, presetActor := range presetActorMap {
		githubActor, exists := githubActorMap[actoryKey]
		if !exists {
			return true
		}

		if githubActor.ActorType == nil || string(*githubActor.ActorType) != presetActor.ActorType {
			return true
		}

		presetMode := presetActor.BypassMode
		githubMode := ""
		if githubActor.BypassMode != nil {
			githubMode = string(*githubActor.BypassMode)
		}
		if presetMode != githubMode {
			return true
		}
	}

	return false
}

// conditionsDiffer compares conditions between preset and GitHub ruleset
func conditionsDiffer(k8sConditions *githubv1alpha1.RulesetConditions, githubConditions *github.RepositoryRulesetConditions) bool {
	if k8sConditions == nil && githubConditions == nil {
		return false
	}
	if k8sConditions == nil || githubConditions == nil {
		return true
	}

	// Compare ref name conditions
	if refNameConditionsDiffer(k8sConditions.RefName, githubConditions.RefName) {
		return true
	}

	// Compare repository name conditions
	if repositoryNameConditionsDiffer(k8sConditions.RepositoryName, githubConditions.RepositoryName) {
		return true
	}

	// Compare repository property conditions
	if repositoryPropertyConditionsDiffer(k8sConditions.RepositoryProperty, githubConditions.RepositoryProperty) {
		return true
	}

	return false
}

// repositoryNameConditionsDiffer compares repository name conditions between preset and GitHub ruleset
func refNameConditionsDiffer(preset *githubv1alpha1.RefNameCondition, gh *github.RepositoryRulesetRefConditionParameters) bool {
	if preset == nil && gh == nil {
		return false
	}
	if preset == nil || gh == nil {
		return true
	}

	if !stringSlicesEqual(preset.Include, gh.Include) {
		return true
	}
	if !stringSlicesEqual(preset.Exclude, gh.Exclude) {
		return true
	}

	return false
}

// repositoryNameConditionsDiffer compares repository name conditions between preset and GitHub ruleset
func repositoryNameConditionsDiffer(preset *githubv1alpha1.RepositoryNameCondition, gh *github.RepositoryRulesetRepositoryNamesConditionParameters) bool {
	if preset == nil && gh == nil {
		return false
	}
	if preset == nil || gh == nil {
		return true
	}

	if !stringSlicesEqual(preset.Include, gh.Include) {
		return true
	}
	if !stringSlicesEqual(preset.Exclude, gh.Exclude) {
		return true
	}
	if utils.WithDefault(gh.Protected, false) != utils.WithDefault(preset.Protected, false) {
		return true
	}

	return false
}

// repositoryPropertyConditionsDiffer compares repository property conditions between preset and GitHub ruleset
func repositoryPropertyConditionsDiffer(preset *githubv1alpha1.RepositoryPropertyCondition, gh *github.RepositoryRulesetRepositoryPropertyConditionParameters) bool {
	if preset == nil && gh == nil {
		return false
	}
	if preset == nil || gh == nil {
		return true
	}

	if len(preset.Include) != len(gh.Include) {
		return true
	}
	if len(preset.Exclude) != len(gh.Exclude) {
		return true
	}

	// Compare include property targets
	if propertyTargetsDiffer(preset.Include, gh.Include) {
		return true
	}
	// Compare exclude property targets
	if propertyTargetsDiffer(preset.Exclude, gh.Exclude) {
		return true
	}

	return false
}

// propertyTargetsDiffer compares repository property target lists
func propertyTargetsDiffer(preset []githubv1alpha1.RepositoryPropertyTarget, gh []*github.RepositoryRulesetRepositoryPropertyTargetParameters) bool {
	presetMap := make(map[string]githubv1alpha1.RepositoryPropertyTarget)
	for _, p := range preset {
		presetMap[p.Name] = p
	}
	ghMap := make(map[string]*github.RepositoryRulesetRepositoryPropertyTargetParameters)
	for _, g := range gh {
		ghMap[g.Name] = g
	}

	for name, presetTarget := range presetMap {
		ghTarget, exists := ghMap[name]
		if !exists {
			return true
		}
		if !stringSlicesEqual(presetTarget.PropertyValues, ghTarget.PropertyValues) {
			return true
		}
		if utils.WithDefault(presetTarget.Source, "custom") != utils.WithDefault(ghTarget.Source, "custom") {
			return true
		}
	}

	return false
}

// rulesetRulesDiffer compares rules between preset and GitHub ruleset
func rulesetRulesDiffer(presetRules githubv1alpha1.RulesetRules, githubRules *github.RepositoryRulesetRules) bool {
	if githubRules == nil {
		return hasAnyRulesSet(presetRules)
	}

	if utils.WithDefault(presetRules.Creation, false) != (githubRules.Creation != nil) {
		return true
	}
	if utils.WithDefault(presetRules.Update, false) != (githubRules.Update != nil) {
		return true
	}
	if utils.WithDefault(presetRules.Deletion, false) != (githubRules.Deletion != nil) {
		return true
	}
	if utils.WithDefault(presetRules.RequiredLinearHistory, false) != (githubRules.RequiredLinearHistory != nil) {
		return true
	}
	if utils.WithDefault(presetRules.NonFastForward, false) != (githubRules.NonFastForward != nil) {
		return true
	}
	if utils.WithDefault(presetRules.RequiredSignatures, false) != (githubRules.RequiredSignatures != nil) {
		return true
	}

	// Compare pull request rules
	if pullRequestRulesDiffer(presetRules.PullRequest, githubRules.PullRequest) {
		return true
	}

	// Compare required status checks rules
	if requiredStatusChecksDiffer(presetRules.RequiredStatusChecks, githubRules.RequiredStatusChecks) {
		return true
	}

	// Compare copilot code review rules
	if copilotCodeReviewDiffer(presetRules.CopilotReview, githubRules.CopilotCodeReview) {
		return true
	}

	// Compare workflow rules
	if workflowsRuleDiffer(presetRules.Workflows, githubRules.Workflows) {
		return true
	}

	// Compare pattern rules
	if patternRulesDiffer(presetRules, githubRules) {
		return true
	}

	return false
}

func copilotCodeReviewDiffer(k8s *githubv1alpha1.CopilotCodeReviewRule, gh *github.CopilotCodeReviewRuleParameters) bool {
	if k8s == nil && gh == nil {
		return false
	}
	if k8s == nil || gh == nil {
		return true
	}

	return utils.WithDefault(k8s.ReviewDraftPullRequests, true) != gh.ReviewDraftPullRequests || utils.WithDefault(k8s.ReviewOnPush, true) != gh.ReviewOnPush
}

// workflowsRuleDiffer compares workflow rules between preset and GitHub ruleset
func workflowsRuleDiffer(k8s *githubv1alpha1.WorkflowsRule, gh *github.WorkflowsRuleParameters) bool {
	if k8s == nil && gh == nil {
		return false
	}
	if k8s == nil || gh == nil {
		return true
	}

	if utils.WithDefault(k8s.DoNotEnforceOnCreate, false) != utils.WithDefault(gh.DoNotEnforceOnCreate, false) {
		return true
	}

	if len(k8s.Workflows) != len(gh.Workflows) {
		return true
	}
	keyFunc := func(id *int64, path string) string {
		return fmt.Sprintf("%d:%s", utils.WithDefault(id, int64(-1)), path)
	}
	k8sMap := make(map[string]githubv1alpha1.RuleWorkflow)
	for _, wf := range k8s.Workflows {
		k8sMap[keyFunc(wf.ResolvedRepositoryID, wf.Path)] = wf
	}

	ghMap := make(map[string]*github.RuleWorkflow)
	for _, wf := range gh.Workflows {
		ghMap[keyFunc(wf.RepositoryID, wf.Path)] = wf
	}

	for key, k8sWf := range k8sMap {
		ghWf, exists := ghMap[key]
		if !exists {
			return true
		}
		// Compare repository ID (resolved from name)
		if !cmp.Equal(k8sWf.ResolvedRepositoryID, ghWf.RepositoryID) {
			return true
		}
		// Compare ref
		k8sRef := ""
		if k8sWf.Ref != nil {
			k8sRef = *k8sWf.Ref
		}
		ghRef := ""
		if ghWf.Ref != nil {
			ghRef = *ghWf.Ref
		}
		if k8sRef != ghRef {
			return true
		}
	}

	return false
}

// pullRequestRulesDiffer compares pull request rules
func pullRequestRulesDiffer(presetPR *githubv1alpha1.PullRequestRule, githubPR *github.PullRequestRuleParameters) bool {
	if presetPR == nil && githubPR == nil {
		return false
	}
	if presetPR == nil || githubPR == nil {
		return true
	}

	if utils.WithDefault(presetPR.DismissStaleReviewsOnPush, false) != githubPR.DismissStaleReviewsOnPush {
		return true
	}
	if utils.WithDefault(presetPR.RequireCodeOwnerReviews, false) != githubPR.RequireCodeOwnerReview {
		return true
	}
	if utils.WithDefault(presetPR.RequireLastPushApproval, false) != githubPR.RequireLastPushApproval {
		return true
	}
	if presetPR.RequiredApprovingReviewCount != githubPR.RequiredApprovingReviewCount {
		return true
	}
	if utils.WithDefault(presetPR.RequiredReviewThreadResolution, false) != githubPR.RequiredReviewThreadResolution {
		return true
	}

	return false
}

// requiredStatusChecksDiffer compares required status checks rules
func requiredStatusChecksDiffer(presetChecks *githubv1alpha1.RequiredStatusChecks, githubChecks *github.RequiredStatusChecksRuleParameters) bool {
	if presetChecks == nil && githubChecks == nil {
		return false
	}
	if presetChecks == nil || githubChecks == nil {
		return true
	}

	if utils.WithDefault(presetChecks.StrictPolicy, false) != githubChecks.StrictRequiredStatusChecksPolicy {
		return true
	}

	// Compare status checks arrays
	if len(presetChecks.Checks) != len(githubChecks.RequiredStatusChecks) {
		return true
	}

	// Create maps for easier comparison
	presetCheckMap := make(map[string]githubv1alpha1.StatusCheck)
	for _, check := range presetChecks.Checks {
		presetCheckMap[check.Context] = check
	}

	githubCheckMap := make(map[string]*github.RuleStatusCheck)
	for _, check := range githubChecks.RequiredStatusChecks {
		githubCheckMap[check.Context] = check
	}

	// Compare each status check
	for context, presetCheck := range presetCheckMap {
		githubCheck, exists := githubCheckMap[context]
		if !exists {
			return true
		}

		// Compare integration ID
		if !cmp.Equal(presetCheck.IntegrationID, githubCheck.IntegrationID) {
			return true
		}
	}

	return false
}

// patternRulesDiffer compares pattern-based rules
func patternRulesDiffer(presetRules githubv1alpha1.RulesetRules, githubRules *github.RepositoryRulesetRules) bool {
	// Compare commit message pattern
	if patternRuleDiffer(presetRules.CommitMessagePattern, githubRules.CommitMessagePattern) {
		return true
	}

	// Compare commit author email pattern
	if patternRuleDiffer(presetRules.CommitAuthorEmailPattern, githubRules.CommitAuthorEmailPattern) {
		return true
	}

	// Compare committer email pattern
	if patternRuleDiffer(presetRules.CommitterEmailPattern, githubRules.CommitterEmailPattern) {
		return true
	}

	// Compare branch name pattern
	if patternRuleDiffer(presetRules.BranchNamePattern, githubRules.BranchNamePattern) {
		return true
	}

	// Compare tag name pattern
	if patternRuleDiffer(presetRules.TagNamePattern, githubRules.TagNamePattern) {
		return true
	}

	return false
}

// patternRuleDiffer compares a single pattern rule
func patternRuleDiffer(presetPattern *githubv1alpha1.PatternRule, githubPattern *github.PatternRuleParameters) bool {
	if presetPattern == nil && githubPattern == nil {
		return false
	}
	if presetPattern == nil || githubPattern == nil {
		return true
	}

	if presetPattern.Pattern != githubPattern.Pattern {
		return true
	}
	if presetPattern.Operator != string(githubPattern.Operator) {
		return true
	}

	presetNegate := utils.WithDefault(presetPattern.Negate, false)
	githubNegate := false
	if githubPattern.Negate != nil {
		githubNegate = *githubPattern.Negate
	}
	if presetNegate != githubNegate {
		return true
	}

	return false
}

// Helper functions

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	mapA := make(map[string]bool)
	for _, s := range a {
		mapA[s] = true
	}

	for _, s := range b {
		if !mapA[s] {
			return false
		}
	}

	return true
}

// hasAnyRulesSet checks if any rules are set in the preset
func hasAnyRulesSet(rules githubv1alpha1.RulesetRules) bool {
	return utils.WithDefault(rules.Creation, false) ||
		utils.WithDefault(rules.Update, false) ||
		utils.WithDefault(rules.Deletion, false) ||
		utils.WithDefault(rules.RequiredLinearHistory, false) ||
		utils.WithDefault(rules.NonFastForward, false) ||
		utils.WithDefault(rules.RequiredSignatures, false) ||
		rules.PullRequest != nil ||
		rules.RequiredStatusChecks != nil ||
		rules.CommitMessagePattern != nil ||
		rules.CommitAuthorEmailPattern != nil ||
		rules.CommitterEmailPattern != nil ||
		rules.BranchNamePattern != nil ||
		rules.TagNamePattern != nil ||
		rules.CopilotReview != nil ||
		rules.Workflows != nil
}
