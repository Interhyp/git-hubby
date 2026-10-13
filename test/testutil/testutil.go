// Package testutil contains helpers shared by unit tests of multiple packages.
package testutil

import (
	"context"
	"sync"

	"github.com/go-logr/logr/funcr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
)

// CapturingContext returns a context with a logger that records all log lines (verbosity <= 1).
// The returned function returns a snapshot of the lines logged so far.
func CapturingContext() (context.Context, func() []string) {
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

// NewFakeClient returns a fake Kubernetes client with the v1alpha1 scheme registered, containing the given objects.
func NewFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}
