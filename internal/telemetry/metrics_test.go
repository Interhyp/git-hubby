package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var _ = Describe("NewMeterProvider", func() {
	It("should expose the otelhttp client metrics via the Prometheus registry", func() {
		registry := prometheus.NewRegistry()
		provider, err := NewMeterProvider(registry)
		Expect(err).NotTo(HaveOccurred())

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		httpClient := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithMeterProvider(provider))}
		resp, err := httpClient.Get(server.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Body.Close()).To(Succeed())

		families, err := registry.Gather()
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(families))
		for _, family := range families {
			names = append(names, family.GetName())
		}
		Expect(names).To(ContainElement("http_client_request_duration_seconds"))
		Expect(names).NotTo(ContainElement("target_info"))

		var labels map[string]string
		for _, family := range families {
			if family.GetName() == "http_client_request_duration_seconds" {
				labels = map[string]string{}
				for _, label := range family.GetMetric()[0].GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
			}
		}
		Expect(labels).To(HaveKeyWithValue("http_request_method", "GET"))
		Expect(labels).To(HaveKeyWithValue("http_response_status_code", "404"))
	})
})

func TestTelemetry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Telemetry Suite")
}
