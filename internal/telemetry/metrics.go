// Package telemetry wires OpenTelemetry instrumentation into the metrics endpoint of the manager.
package telemetry

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// NewMeterProvider creates an OpenTelemetry MeterProvider whose metrics are exposed via the given Prometheus
// registerer. Pass the controller-runtime registry (sigs.k8s.io/controller-runtime/pkg/metrics.Registry) so that the
// metrics are served by the existing metrics endpoint of the manager alongside the controller-runtime metrics.
func NewMeterProvider(registerer prometheus.Registerer) (metric.MeterProvider, error) {
	exporter, err := otelprometheus.New(
		otelprometheus.WithRegisterer(registerer),
		// target_info and the otel_scope_* labels add no information for a single binary scraped by one job
		otelprometheus.WithoutTargetInfo(),
		otelprometheus.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenTelemetry Prometheus exporter: %w", err)
	}
	return sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)), nil
}
