package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v92/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileWebhooks with inline webhooks", func() {
	var (
		ctx          context.Context
		mockClient   *ghclientmock.MockGitHubClientWrapper
		repo         *v1alpha1.Repository
		objects      []client.Object
		err          error
		listCalled   bool
		existing     []*github.Hook
		createdHooks []*github.Hook
		deletedIDs   []int64
	)

	inlineWebhook := func(url string) v1alpha1.WebhookPresetSpec {
		return v1alpha1.WebhookPresetSpec{
			PayloadURL:  url,
			ContentType: "json",
			Active:      new(true),
			Events:      []string{"push"},
			SSLVerify:   new(true),
		}
	}
	ghHook := func(id int64, url string) *github.Hook {
		return &github.Hook{
			ID:     new(id),
			Config: &github.HookConfig{URL: new(url), ContentType: new("json")},
			Events: []string{"push"},
		}
	}
	secret := func(namespace, name, key, value string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Data:       map[string][]byte{key: []byte(value)},
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{Name: "test-repo", Namespace: "default"},
			Spec:       v1alpha1.RepositorySpec{Name: "test-repo"},
			Status:     v1alpha1.RepositoryStatus{Webhooks: map[string]v1alpha1.WebhookStatus{}},
		}
		objects = []client.Object{
			&v1alpha1.WebhookPreset{
				ObjectMeta: metav1.ObjectMeta{Name: "preset-1", Namespace: "default"},
				Spec:       inlineWebhook("https://preset.example.com/hook"),
			},
		}
		listCalled = false
		existing = nil
		createdHooks = nil
		deletedIDs = nil

		mockClient.GetRepositoryFunc = func(_ context.Context, owner, name string) (*github.Repository, error) {
			return &github.Repository{Name: new(name), Owner: &github.User{Login: new(owner)}}, nil
		}
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(objects, repo)...).Build()

		mockClient.ListHooksFunc = func(_ context.Context, _, _ string, _ *github.ListOptions) ([]*github.Hook, error) {
			listCalled = true
			return existing, nil
		}
		mockClient.CreateHookFunc = func(_ context.Context, _, _ string, hook *github.Hook) (*github.Hook, error) {
			createdHooks = append(createdHooks, hook)
			return hook, nil
		}
		mockClient.DeleteHookFunc = func(_ context.Context, _, _ string, id int64) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		}

		rec := &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client:   mockClient,
				Resource: GitHubRepoIdentifier{Owner: "test-org", Name: "test-repo"},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{Client: k8sClient, Resource: repo},
		}
		err = rec.reconcileWebhooks(ctx)
	})

	Context("when an inline webhook does not exist on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{inlineWebhook("https://inline.example.com/hook")}
		})

		It("creates it and tracks it in the status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdHooks).To(HaveLen(1))
			Expect(*createdHooks[0].Config.URL).To(Equal("https://inline.example.com/hook"))
			Expect(repo.Status.Webhooks).To(HaveKey(mapper.HashWebhookConfig("https://inline.example.com/hook", "json", []string{"push"})))
		})
	})

	Context("when an inline webhook already exists on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{inlineWebhook("https://inline.example.com/hook")}
			existing = []*github.Hook{ghHook(1, "https://inline.example.com/hook")}
		})

		It("does nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdHooks).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when an inline webhook was removed from the spec", func() {
		BeforeEach(func() {
			existing = []*github.Hook{ghHook(1, "https://inline.example.com/hook")}
		})

		It("deletes the orphaned webhook", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedIDs).To(ConsistOf(int64(1)))
		})
	})

	Context("when inline webhooks and presets are combined", func() {
		BeforeEach(func() {
			repo.Spec.WebhookPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{inlineWebhook("https://inline.example.com/hook")}
		})

		It("creates the union", func() {
			Expect(err).NotTo(HaveOccurred())
			urls := []string{}
			for _, h := range createdHooks {
				urls = append(urls, *h.Config.URL)
			}
			Expect(urls).To(ConsistOf("https://preset.example.com/hook", "https://inline.example.com/hook"))
		})
	})

	Context("when an inline webhook uses a templated payload URL and a secret", func() {
		BeforeEach(func() {
			hook := inlineWebhook("https://inline.example.com/{{ .Name }}")
			hook.Secret = &v1alpha1.WebhookPresetSecretSpec{Name: new("hook-secret"), Key: new("token")}
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{hook}
			objects = append(objects, secret("default", "hook-secret", "token", "s3cr3t"))
		})

		It("creates the templated webhook with the secret", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdHooks).To(HaveLen(1))
			Expect(*createdHooks[0].Config.URL).To(Equal("https://inline.example.com/test-repo"))
			Expect(*createdHooks[0].Config.Secret).To(Equal("s3cr3t"))
			hash := mapper.HashWebhookConfig("https://inline.example.com/test-repo", "json", []string{"push"})
			Expect(repo.Status.Webhooks[hash].SecretHash).To(Equal(sha([]byte("s3cr3t"))))
		})

		It("does not write the templated URL or the secret back into the spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Spec.Webhooks[0].PayloadURL).To(Equal("https://inline.example.com/{{ .Name }}"))
			Expect(repo.Spec.Webhooks[0].SecretValue).To(BeNil())
		})
	})

	Context("when the secret of an inline webhook rotates", func() {
		BeforeEach(func() {
			hook := inlineWebhook("https://inline.example.com/hook")
			hook.Secret = &v1alpha1.WebhookPresetSecretSpec{Name: new("hook-secret"), Key: new("token")}
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{hook}
			objects = append(objects, secret("default", "hook-secret", "token", "new-value"))
			existing = []*github.Hook{ghHook(1, "https://inline.example.com/hook")}
			repo.Status.Webhooks[mapper.HashWebhookConfig("https://inline.example.com/hook", "json", []string{"push"})] =
				v1alpha1.WebhookStatus{SecretHash: sha([]byte("old-value"))}
		})

		It("recreates the webhook", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedIDs).To(ConsistOf(int64(1)))
			Expect(createdHooks).To(HaveLen(1))
		})
	})

	Context("when an inline webhook references a secret in another namespace", func() {
		BeforeEach(func() {
			hook := inlineWebhook("https://inline.example.com/hook")
			hook.Secret = &v1alpha1.WebhookPresetSecretSpec{Name: new("hook-secret"), Key: new("token"), Namespace: new("other")}
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{hook}
			objects = append(objects, secret("other", "hook-secret", "token", "s3cr3t"))
		})

		It("rejects the secret reference and makes no GitHub calls", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("outside of the repository namespace"))
			Expect(listCalled).To(BeFalse())
			Expect(createdHooks).To(BeEmpty())
		})
	})

	Context("when an inline webhook conflicts with a preset", func() {
		BeforeEach(func() {
			repo.Spec.WebhookPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			conflicting := inlineWebhook("https://preset.example.com/hook")
			conflicting.Active = new(false)
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{conflicting}
			existing = []*github.Hook{ghHook(1, "https://orphan.example.com/hook")}
		})

		It("returns a conflict error and makes no GitHub calls", func() {
			var conflictErr *reconciler.WebhookConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Sources).To(ConsistOf("webhookPreset/preset-1", "inline[0]"))
			Expect(listCalled).To(BeFalse())
			Expect(createdHooks).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when an inline webhook is identical to a preset", func() {
		BeforeEach(func() {
			repo.Spec.WebhookPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Webhooks = []v1alpha1.WebhookPresetSpec{inlineWebhook("https://preset.example.com/hook")}
		})

		It("creates it once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdHooks).To(HaveLen(1))
		})
	})

	Context("when inline ignore rules match existing webhooks", func() {
		BeforeEach(func() {
			repo.Spec.WebhookIgnores = []v1alpha1.WebhookIgnorePresetSpec{{IgnoreURLRegex: new("^https://ignored\\.example\\.com/.*")}}
			existing = []*github.Hook{
				ghHook(1, "https://ignored.example.com/hook"),
				ghHook(2, "https://orphan.example.com/hook"),
			}
		})

		It("keeps the ignored webhooks and deletes only orphans", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedIDs).To(ConsistOf(int64(2)))
		})
	})
})
