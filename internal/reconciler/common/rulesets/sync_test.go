package rulesets

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/merge"
	"github.com/google/go-github/v92/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeRulesetResolver records every ruleset passed to ResolveRuleset and returns it unchanged (together with err).
type fakeRulesetResolver struct {
	err   error
	calls []v1alpha1.RulesetSpec
}

func (f *fakeRulesetResolver) ResolveRuleset(_ context.Context, rs v1alpha1.RulesetSpec) (v1alpha1.RulesetSpec, error) {
	f.calls = append(f.calls, *rs.DeepCopy())
	return rs, f.err
}

func (f *fakeRulesetResolver) calledNames() []string {
	names := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		names = append(names, c.Name)
	}
	return names
}

// fakeRulesetTarget is an in-memory Target recording all calls made by Sync.
type fakeRulesetTarget struct {
	existing []*github.RepositoryRuleset

	listErr, getErr, createErr, updateErr, deleteErr error

	listCalled     bool
	getCalls       []int64
	created        []*github.RepositoryRuleset
	createdDesired []Desired
	updated        map[int64]*github.RepositoryRuleset
	deleted        map[int64]string
}

func newFakeRulesetTarget() *fakeRulesetTarget {
	return &fakeRulesetTarget{
		updated: map[int64]*github.RepositoryRuleset{},
		deleted: map[int64]string{},
	}
}

func (f *fakeRulesetTarget) target() Target {
	return Target{
		Scope: "test",
		List: func(_ context.Context) ([]*github.RepositoryRuleset, error) {
			f.listCalled = true
			return f.existing, f.listErr
		},
		Get: func(_ context.Context, id int64) (*github.RepositoryRuleset, error) {
			f.getCalls = append(f.getCalls, id)
			if f.getErr != nil {
				return nil, f.getErr
			}
			for _, rs := range f.existing {
				if rs != nil && rs.ID != nil && *rs.ID == id {
					return rs, nil
				}
			}
			return nil, errors.New("ruleset not found")
		},
		Create: func(_ context.Context, rs *github.RepositoryRuleset, desired Desired) error {
			if f.createErr != nil {
				return f.createErr
			}
			f.created = append(f.created, rs)
			f.createdDesired = append(f.createdDesired, desired)
			return nil
		},
		Update: func(_ context.Context, id int64, rs *github.RepositoryRuleset, _ Desired) error {
			if f.updateErr != nil {
				return f.updateErr
			}
			f.updated[id] = rs
			return nil
		},
		Delete: func(_ context.Context, id int64, name string) error {
			if f.deleteErr != nil {
				return f.deleteErr
			}
			f.deleted[id] = name
			return nil
		},
	}
}

func (f *fakeRulesetTarget) createdNames() []string {
	names := make([]string, 0, len(f.created))
	for _, rs := range f.created {
		names = append(names, rs.Name)
	}
	return names
}

var _ = Describe("Sync", func() {
	var (
		ctx      context.Context
		target   *fakeRulesetTarget
		resolver *fakeRulesetResolver
		opts     SyncOptions
		desired  []Desired
		err      error
	)

	// desiredSpec returns a branch ruleset that matches githubRuleset(_, name).
	desiredSpec := func(name string) v1alpha1.RulesetSpec {
		return v1alpha1.RulesetSpec{
			Name:        name,
			Target:      "branch",
			Enforcement: v1alpha1.RulesetEnforcementActive,
			Conditions: &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{Include: []string{"main"}},
			},
			Rules: v1alpha1.RulesetRules{RequiredLinearHistory: new(true)},
		}
	}

	githubRuleset := func(id int64, name string) *github.RepositoryRuleset {
		return &github.RepositoryRuleset{
			ID:          new(id),
			Name:        name,
			Enforcement: github.RulesetEnforcementActive,
			Target:      new(github.RulesetTargetBranch),
			Conditions: &github.RepositoryRulesetConditions{
				RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
			},
			Rules: &github.RepositoryRulesetRules{RequiredLinearHistory: &github.EmptyRuleParameters{}},
		}
	}

	inlineDesired := func(specs ...v1alpha1.RulesetSpec) []Desired {
		result := make([]Desired, 0, len(specs))
		for i, s := range specs {
			result = append(result, Desired{Source: merge.InlineSource(i), Spec: s})
		}
		return result
	}

	BeforeEach(func() {
		ctx = context.Background()
		target = newFakeRulesetTarget()
		resolver = &fakeRulesetResolver{}
		opts = SyncOptions{Resolver: resolver}
		desired = nil
	})

	JustBeforeEach(func() {
		err = Sync(ctx, target.target(), desired, opts)
	})

	Context("with neither desired nor existing rulesets", func() {
		It("makes no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(target.listCalled).To(BeTrue())
			Expect(target.created).To(BeEmpty())
			Expect(target.updated).To(BeEmpty())
			Expect(target.deleted).To(BeEmpty())
			Expect(resolver.calls).To(BeEmpty())
		})
	})

	Context("when desired rulesets do not exist on GitHub", func() {
		BeforeEach(func() {
			desired = inlineDesired(desiredSpec("ruleset-a"), desiredSpec("ruleset-b"))
		})

		It("creates them with the mapped GitHub ruleset and the desired source", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(target.createdNames()).To(Equal([]string{"ruleset-a", "ruleset-b"}))
			Expect(target.created[0].Enforcement).To(Equal(github.RulesetEnforcementActive))
			Expect(target.created[0].Conditions.RefName.Include).To(ConsistOf("main"))
			Expect(target.createdDesired[0].Source).To(Equal("inline[0]"))
			Expect(target.createdDesired[1].Source).To(Equal("inline[1]"))
			Expect(target.getCalls).To(BeEmpty())
			Expect(target.updated).To(BeEmpty())
			Expect(target.deleted).To(BeEmpty())
		})

		It("resolves every desired ruleset exactly once", func() {
			Expect(resolver.calledNames()).To(Equal([]string{"ruleset-a", "ruleset-b"}))
		})
	})

	DescribeTable("when a desired ruleset already exists and matches",
		func(adjustDesired func(*v1alpha1.RulesetSpec), adjustExisting func(*github.RepositoryRuleset)) {
			spec := desiredSpec("ruleset-a")
			existing := githubRuleset(100, "ruleset-a")
			if adjustDesired != nil {
				adjustDesired(&spec)
			}
			if adjustExisting != nil {
				adjustExisting(existing)
			}
			target.existing = []*github.RepositoryRuleset{existing}

			Expect(Sync(ctx, target.target(), inlineDesired(spec), opts)).To(Succeed())
			Expect(target.getCalls).To(ConsistOf(int64(100)))
			Expect(target.created).To(BeEmpty())
			Expect(target.updated).To(BeEmpty())
			Expect(target.deleted).To(BeEmpty())
		},
		Entry("plain ruleset", nil, nil),
		Entry("with copilot code review rule",
			func(s *v1alpha1.RulesetSpec) {
				s.Rules.CopilotReview = &v1alpha1.CopilotCodeReviewRule{ReviewOnPush: new(true), ReviewDraftPullRequests: new(true)}
			},
			func(rs *github.RepositoryRuleset) {
				rs.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{ReviewOnPush: true, ReviewDraftPullRequests: true}
			}),
	)

	DescribeTable("when a desired ruleset already exists but differs",
		func(adjustDesired func(*v1alpha1.RulesetSpec), adjustExisting func(*github.RepositoryRuleset), verify func(*github.RepositoryRuleset)) {
			spec := desiredSpec("ruleset-a")
			existing := githubRuleset(100, "ruleset-a")
			if adjustDesired != nil {
				adjustDesired(&spec)
			}
			if adjustExisting != nil {
				adjustExisting(existing)
			}
			target.existing = []*github.RepositoryRuleset{existing}

			Expect(Sync(ctx, target.target(), inlineDesired(spec), opts)).To(Succeed())
			Expect(target.getCalls).To(ConsistOf(int64(100)))
			Expect(target.created).To(BeEmpty())
			Expect(target.deleted).To(BeEmpty())
			Expect(target.updated).To(HaveLen(1))
			Expect(target.updated).To(HaveKey(int64(100)))
			Expect(target.updated[100].Name).To(Equal("ruleset-a"))
			verify(target.updated[100])
		},
		Entry("enforcement",
			func(s *v1alpha1.RulesetSpec) { s.Enforcement = v1alpha1.RulesetEnforcementEvaluate }, nil,
			func(rs *github.RepositoryRuleset) {
				Expect(rs.Enforcement).To(Equal(github.RulesetEnforcementEvaluate))
			}),
		Entry("rules",
			func(s *v1alpha1.RulesetSpec) { s.Rules.RequiredSignatures = new(true) }, nil,
			func(rs *github.RepositoryRuleset) { Expect(rs.Rules.RequiredSignatures).NotTo(BeNil()) }),
		Entry("ref name conditions",
			func(s *v1alpha1.RulesetSpec) { s.Conditions.RefName.Include = []string{"main", "develop"} }, nil,
			func(rs *github.RepositoryRuleset) {
				Expect(rs.Conditions.RefName.Include).To(ConsistOf("main", "develop"))
			}),
		Entry("bypass actors",
			func(s *v1alpha1.RulesetSpec) {
				s.BypassActors = []v1alpha1.RulesetBypassActor{{ActorID: new(int64(999)), ActorType: "Team", BypassMode: "always"}}
			},
			func(rs *github.RepositoryRuleset) {
				rs.BypassActors = []*github.BypassActor{{ActorID: new(int64(888)), ActorType: new(github.BypassActorTypeTeam)}}
			},
			func(rs *github.RepositoryRuleset) {
				Expect(rs.BypassActors).To(HaveLen(1))
				Expect(rs.BypassActors[0].GetActorID()).To(Equal(int64(999)))
			}),
		Entry("copilot code review rule settings",
			func(s *v1alpha1.RulesetSpec) {
				s.Rules.CopilotReview = &v1alpha1.CopilotCodeReviewRule{ReviewOnPush: new(true), ReviewDraftPullRequests: new(true)}
			},
			func(rs *github.RepositoryRuleset) {
				rs.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{ReviewOnPush: false, ReviewDraftPullRequests: true}
			},
			func(rs *github.RepositoryRuleset) {
				Expect(rs.Rules.CopilotCodeReview).NotTo(BeNil())
				Expect(rs.Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
				Expect(rs.Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeTrue())
			}),
		Entry("removed copilot code review rule", nil,
			func(rs *github.RepositoryRuleset) {
				rs.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{ReviewOnPush: true, ReviewDraftPullRequests: true}
			},
			func(rs *github.RepositoryRuleset) { Expect(rs.Rules.CopilotCodeReview).To(BeNil()) }),
	)

	Context("when existing rulesets are not desired", func() {
		BeforeEach(func() {
			desired = inlineDesired(desiredSpec("keep"))
			target.existing = []*github.RepositoryRuleset{
				githubRuleset(100, "keep"),
				githubRuleset(200, "orphan-1"),
				githubRuleset(300, "orphan-2"),
			}
		})

		It("deletes only the orphaned rulesets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(target.deleted).To(Equal(map[int64]string{200: "orphan-1", 300: "orphan-2"}))
			Expect(target.created).To(BeEmpty())
			Expect(target.updated).To(BeEmpty())
		})
	})

	Context("when rulesets need to be created, updated and deleted", func() {
		BeforeEach(func() {
			changed := desiredSpec("update-me")
			changed.Enforcement = v1alpha1.RulesetEnforcementEvaluate
			desired = inlineDesired(desiredSpec("create-me"), changed)
			target.existing = []*github.RepositoryRuleset{
				githubRuleset(200, "update-me"),
				githubRuleset(300, "delete-me"),
			}
		})

		It("applies all changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(target.createdNames()).To(Equal([]string{"create-me"}))
			Expect(target.updated).To(HaveLen(1))
			Expect(target.updated).To(HaveKey(int64(200)))
			Expect(target.deleted).To(Equal(map[int64]string{300: "delete-me"}))
		})
	})

	Context("when existing rulesets are incomplete", func() {
		BeforeEach(func() {
			noID := githubRuleset(0, "no-id-orphan")
			noID.ID = nil
			target.existing = []*github.RepositoryRuleset{nil, githubRuleset(100, ""), noID}
		})

		It("ignores nil entries, empty names and orphans without ID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(target.deleted).To(BeEmpty())
		})
	})

	Context("when an existing ruleset matching a desired name has no ID", func() {
		BeforeEach(func() {
			changed := desiredSpec("ruleset-a")
			changed.Enforcement = v1alpha1.RulesetEnforcementEvaluate
			desired = inlineDesired(changed)
			existing := githubRuleset(0, "ruleset-a")
			existing.ID = nil
			target.existing = []*github.RepositoryRuleset{existing}
		})

		It("skips the ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(target.getCalls).To(BeEmpty())
			Expect(target.created).To(BeEmpty())
			Expect(target.updated).To(BeEmpty())
			Expect(target.deleted).To(BeEmpty())
		})
	})

	Context("error handling", func() {
		BeforeEach(func() {
			desired = inlineDesired(desiredSpec("ruleset-a"))
		})

		Context("when listing the existing rulesets fails", func() {
			BeforeEach(func() {
				target.listErr = errors.New("list failed")
			})

			It("returns the error without processing rulesets", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get existing test rulesets")))
				Expect(errors.Is(err, target.listErr)).To(BeTrue())
				Expect(resolver.calls).To(BeEmpty())
				Expect(target.created).To(BeEmpty())
			})
		})

		Context("when fetching an existing ruleset fails", func() {
			BeforeEach(func() {
				target.existing = []*github.RepositoryRuleset{githubRuleset(100, "ruleset-a")}
				target.getErr = errors.New("get failed")
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get existing test ruleset ruleset-a")))
				Expect(errors.Is(err, target.getErr)).To(BeTrue())
				Expect(target.updated).To(BeEmpty())
			})
		})

		Context("when creating a ruleset fails", func() {
			BeforeEach(func() {
				target.createErr = errors.New("create failed")
			})

			It("returns the error", func() {
				Expect(errors.Is(err, target.createErr)).To(BeTrue())
			})
		})

		Context("when updating a ruleset fails", func() {
			BeforeEach(func() {
				desired[0].Spec.Enforcement = v1alpha1.RulesetEnforcementEvaluate
				target.existing = []*github.RepositoryRuleset{githubRuleset(100, "ruleset-a")}
				target.updateErr = errors.New("update failed")
			})

			It("returns the error", func() {
				Expect(errors.Is(err, target.updateErr)).To(BeTrue())
			})
		})

		Context("when deleting a ruleset fails", func() {
			BeforeEach(func() {
				desired = nil
				target.existing = []*github.RepositoryRuleset{githubRuleset(100, "orphan")}
				target.deleteErr = errors.New("delete failed")
			})

			It("returns the error", func() {
				Expect(errors.Is(err, target.deleteErr)).To(BeTrue())
			})
		})

		Context("when the ruleset cannot be mapped to a GitHub ruleset", func() {
			BeforeEach(func() {
				desired[0].Spec.Name = ""
			})

			It("returns a mapping error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to convert ruleset")))
				Expect(target.created).To(BeEmpty())
			})
		})

		Context("when resolving the ruleset fails", func() {
			BeforeEach(func() {
				desired = inlineDesired(desiredSpec("ruleset-a"), desiredSpec("ruleset-b"))
				resolver.err = errors.New("unknown team")
			})

			It("returns the error before any change is made", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to resolve slugs of ruleset ruleset-a (inline[0])")))
				Expect(errors.Is(err, resolver.err)).To(BeTrue())
				Expect(resolver.calledNames()).To(Equal([]string{"ruleset-a"}))
				Expect(target.created).To(BeEmpty())
				Expect(target.updated).To(BeEmpty())
				Expect(target.deleted).To(BeEmpty())
			})
		})
	})

	Context("required reviewers feature flag", func() {
		BeforeEach(func() {
			spec := desiredSpec("ruleset-a")
			spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
				RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
					{Reviewer: v1alpha1.PullRequestReviewerEntity{ID: new(int64(1)), Type: "Team"}, FilePatterns: []string{"*"}},
				},
			}
			desired = inlineDesired(spec)
		})

		Context("when disabled", func() {
			It("strips required reviewers before resolving", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resolver.calls).To(HaveLen(1))
				Expect(resolver.calls[0].Rules.PullRequest.RequiredReviewers).To(BeNil())
			})
		})

		Context("when enabled", func() {
			BeforeEach(func() {
				opts.EnableRequiredReviewersRules = true
			})

			It("keeps required reviewers", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resolver.calls).To(HaveLen(1))
				Expect(resolver.calls[0].Rules.PullRequest.RequiredReviewers).To(HaveLen(1))
			})
		})
	})

	Context("with a Skip option", func() {
		BeforeEach(func() {
			desired = inlineDesired(desiredSpec("skipped"), desiredSpec("applied"))
			target.existing = []*github.RepositoryRuleset{githubRuleset(100, "skipped")}
			opts.Skip = func(_ context.Context, d Desired) bool { return d.Spec.Name == "skipped" }
		})

		It("does not resolve or apply skipped rulesets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(resolver.calledNames()).To(Equal([]string{"applied"}))
			Expect(target.createdNames()).To(Equal([]string{"applied"}))
			Expect(target.getCalls).To(BeEmpty())
		})

		It("deletes an existing ruleset named like a skipped ruleset", func() {
			Expect(target.deleted).To(Equal(map[int64]string{100: "skipped"}))
		})
	})

	Context("with an Adjust option", func() {
		BeforeEach(func() {
			desired = inlineDesired(desiredSpec("ruleset-a"))
			opts.Adjust = func(spec *v1alpha1.RulesetSpec) {
				spec.Enforcement = v1alpha1.RulesetEnforcementDisabled
			}
		})

		It("adjusts the ruleset after resolving and before mapping", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(resolver.calls).To(HaveLen(1))
			Expect(resolver.calls[0].Enforcement).To(Equal(v1alpha1.RulesetEnforcementActive))
			Expect(target.created).To(HaveLen(1))
			Expect(target.created[0].Enforcement).To(Equal(github.RulesetEnforcementDisabled))
		})

		Context("when the existing ruleset matches the adjusted ruleset", func() {
			BeforeEach(func() {
				existing := githubRuleset(100, "ruleset-a")
				existing.Enforcement = github.RulesetEnforcementDisabled
				target.existing = []*github.RepositoryRuleset{existing}
			})

			It("compares against the adjusted ruleset and skips the update", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(target.getCalls).To(ConsistOf(int64(100)))
				Expect(target.updated).To(BeEmpty())
			})
		})
	})
})
