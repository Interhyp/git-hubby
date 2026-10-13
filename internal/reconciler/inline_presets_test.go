package reconciler

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// capturingContext returns a context with a logger that records all log lines in the returned slice pointer.
func capturingContext() (context.Context, func() []string) {
	var (
		mu    sync.Mutex
		lines []string
	)
	logger := funcr.New(func(_, args string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, args)
	}, funcr.Options{Verbosity: 1})
	return logPkg.IntoContext(context.Background(), logger), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), lines...)
	}
}

func newFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

var _ = Describe("CollectDesiredAutolinks", func() {
	const namespace = "test-ns"

	var (
		ctx        context.Context
		logs       func() []string
		presets    []client.Object
		presetRefs []corev1.LocalObjectReference
		inline     []v1alpha1.Autolink
		result     []DesiredAutolink
		err        error
	)

	autolink := func(prefix, url string) v1alpha1.Autolink {
		return v1alpha1.Autolink{KeyPrefix: prefix, URLTemplate: url, IsAlphanumeric: true}
	}
	newPreset := func(name string, autolinks ...v1alpha1.Autolink) *v1alpha1.AutolinksPreset {
		return &v1alpha1.AutolinksPreset{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       v1alpha1.AutolinksPresetSpec{AutolinkList: autolinks},
		}
	}

	BeforeEach(func() {
		ctx, logs = capturingContext()
		presets = nil
		presetRefs = nil
		inline = nil
	})

	JustBeforeEach(func() {
		result, err = CollectDesiredAutolinks(ctx, newFakeClient(presets...), namespace, presetRefs, inline)
	})

	Context("with presets only", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("p1", autolink("A-", "https://a/<num>"), autolink("B-", "https://b/<num>"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "p1"}}
		})

		It("returns all autolinks with their sources", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Source).To(Equal("autolinksPreset/p1[0]"))
			Expect(result[1].Source).To(Equal("autolinksPreset/p1[1]"))
		})
	})

	Context("with inline autolinks only", func() {
		BeforeEach(func() {
			inline = []v1alpha1.Autolink{autolink("A-", "https://a/<num>")}
		})

		It("returns the inline autolinks", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Source).To(Equal("inline[0]"))
			Expect(result[0].Spec.KeyPrefix).To(Equal("A-"))
		})
	})

	Context("with presets and inline autolinks", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("p1", autolink("A-", "https://a/<num>"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "p1"}}
			inline = []v1alpha1.Autolink{autolink("B-", "https://b/<num>")}
		})

		It("returns the union", func() {
			Expect(err).NotTo(HaveOccurred())
			prefixes := []string{}
			for _, r := range result {
				prefixes = append(prefixes, r.Spec.KeyPrefix)
			}
			Expect(prefixes).To(ConsistOf("A-", "B-"))
		})
	})

	Context("with an identical duplicate", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("p1", autolink("A-", "https://a/<num>"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "p1"}}
			inline = []v1alpha1.Autolink{autolink("A-", "https://a/<num>")}
		})

		It("returns the autolink once and logs a warning", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(strings.Join(logs(), "\n")).To(ContainSubstring("duplicate autolink definition with identical content"))
		})
	})

	Context("with a conflicting inline autolink", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("p1", autolink("A-", "https://a/<num>"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "p1"}}
			inline = []v1alpha1.Autolink{autolink("A-", "https://other/<num>")}
		})

		It("returns a conflict error naming both sources", func() {
			Expect(result).To(BeNil())
			var conflictErr *AutolinkConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.KeyPrefix).To(Equal("A-"))
			Expect(conflictErr.Sources).To(ConsistOf("autolinksPreset/p1[0]", "inline[0]"))
		})
	})

	Context("with conflicting presets", func() {
		BeforeEach(func() {
			presets = []client.Object{
				newPreset("p1", autolink("A-", "https://a/<num>")),
				newPreset("p2", autolink("A-", "https://other/<num>")),
			}
			presetRefs = []corev1.LocalObjectReference{{Name: "p1"}, {Name: "p2"}}
		})

		It("returns a conflict error naming both presets", func() {
			var conflictErr *AutolinkConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Sources).To(ConsistOf("autolinksPreset/p1[0]", "autolinksPreset/p2[0]"))
		})
	})

	Context("with a missing preset", func() {
		BeforeEach(func() {
			presetRefs = []corev1.LocalObjectReference{{Name: "missing"}}
		})

		It("returns a not found error", func() {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("missing"))
		})
	})
})

var _ = Describe("DedupeDesiredWebhooks", func() {
	var (
		ctx  context.Context
		logs func() []string
	)

	webhook := func(source, hash string, active bool) DesiredWebhook {
		return DesiredWebhook{
			Source: source,
			Hash:   hash,
			Spec: v1alpha1.WebhookPresetSpec{
				PayloadURL:  "https://example.com/hook",
				ContentType: "json",
				Active:      new(active),
			},
		}
	}

	BeforeEach(func() {
		ctx, logs = capturingContext()
	})

	It("keeps webhooks with different hashes", func() {
		result, err := DedupeDesiredWebhooks(ctx, []DesiredWebhook{webhook("a", "h1", true), webhook("b", "h2", true)})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(HaveLen(2))
	})

	It("applies identical duplicates once and logs a warning", func() {
		result, err := DedupeDesiredWebhooks(ctx, []DesiredWebhook{webhook("webhookPreset/p1", "h1", true), webhook("inline[0]", "h1", true)})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(HaveLen(1))
		Expect(result[0].Source).To(Equal("webhookPreset/p1"))
		Expect(strings.Join(logs(), "\n")).To(ContainSubstring("duplicate webhook definition with identical content"))
	})

	It("reports conflicting duplicates", func() {
		result, err := DedupeDesiredWebhooks(ctx, []DesiredWebhook{webhook("webhookPreset/p1", "h1", true), webhook("inline[0]", "h1", false)})
		Expect(result).To(BeNil())
		var conflictErr *WebhookConflictError
		Expect(errors.As(err, &conflictErr)).To(BeTrue())
		Expect(conflictErr.URL).To(Equal("https://example.com/hook"))
		Expect(conflictErr.Sources).To(ConsistOf("webhookPreset/p1", "inline[0]"))
	})
})

var _ = Describe("CollectWebhookIgnoreRegexes", func() {
	const namespace = "test-ns"

	newPreset := func(name string, regex *string) *v1alpha1.WebhookIgnorePreset {
		return &v1alpha1.WebhookIgnorePreset{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       v1alpha1.WebhookIgnorePresetSpec{IgnoreURLRegex: regex},
		}
	}

	It("returns the union of preset and inline regexes, compiling duplicates once and skipping empty entries", func() {
		k8sClient := newFakeClient(newPreset("p1", new("^https://a/.*")), newPreset("p2", nil))
		regexes, err := CollectWebhookIgnoreRegexes(context.Background(), k8sClient, namespace,
			[]corev1.LocalObjectReference{{Name: "p1"}, {Name: "p2"}},
			[]v1alpha1.WebhookIgnorePresetSpec{
				{IgnoreURLRegex: new("^https://a/.*")},
				{IgnoreURLRegex: new("")},
				{IgnoreURLRegex: new("^https://b/.*")},
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(regexes).To(HaveLen(2))
		Expect(regexes[0].String()).To(Equal("^https://a/.*"))
		Expect(regexes[1].String()).To(Equal("^https://b/.*"))
	})

	It("returns an error for invalid regexes", func() {
		_, err := CollectWebhookIgnoreRegexes(context.Background(), newFakeClient(), namespace, nil,
			[]v1alpha1.WebhookIgnorePresetSpec{{IgnoreURLRegex: new("(")}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("inline[0]"))
	})

	It("returns an error for missing presets", func() {
		_, err := CollectWebhookIgnoreRegexes(context.Background(), newFakeClient(), namespace,
			[]corev1.LocalObjectReference{{Name: "missing"}}, nil)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
})
