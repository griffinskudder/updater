package observability

import (
	"context"
	"testing"
	"updater/internal/models"
	"updater/internal/version"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_MetricsOnly(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: true,
		Path:    "/metrics",
		Port:    9090,
	}
	obs := models.ObservabilityConfig{
		ServiceName: "test-service",
		Tracing: models.TracingConfig{
			Enabled: false,
		},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.NotNil(t, provider.promExporter)
	assert.Nil(t, provider.tracerProvider)

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSetup_TracingStdout(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: false,
	}
	obs := models.ObservabilityConfig{
		ServiceName: "test-service",
		Tracing: models.TracingConfig{
			Enabled:    true,
			Exporter:   "stdout",
			SampleRate: 1.0,
		},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.NotNil(t, provider.tracerProvider)
	assert.Nil(t, provider.promExporter)

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSetup_BothEnabled(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: true,
		Path:    "/metrics",
		Port:    9090,
	}
	obs := models.ObservabilityConfig{
		ServiceName: "test-service",
		Tracing: models.TracingConfig{
			Enabled:    true,
			Exporter:   "stdout",
			SampleRate: 0.5,
		},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.NotNil(t, provider.tracerProvider)
	assert.NotNil(t, provider.promExporter)

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSetup_BothDisabled(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: false,
	}
	obs := models.ObservabilityConfig{
		Tracing: models.TracingConfig{
			Enabled: false,
		},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Nil(t, provider.tracerProvider)
	assert.Nil(t, provider.promExporter)

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSetup_InvalidExporter(t *testing.T) {
	metrics := models.MetricsConfig{Enabled: false}
	obs := models.ObservabilityConfig{
		ServiceName: "test-service",
		Tracing: models.TracingConfig{
			Enabled:    true,
			Exporter:   "invalid",
			SampleRate: 1.0,
		},
	}

	provider, err := Setup(metrics, obs, version.Info{})
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "unsupported trace exporter")
}

func TestSetup_SamplerConfigurations(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"ratio based", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := models.MetricsConfig{Enabled: false}
			obs := models.ObservabilityConfig{
				ServiceName: "test",
				Tracing: models.TracingConfig{
					Enabled:    true,
					Exporter:   "stdout",
					SampleRate: tt.sampleRate,
				},
			}

			provider, err := Setup(metrics, obs, version.Info{})
			require.NoError(t, err)
			require.NotNil(t, provider)

			err = provider.Shutdown(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestProvider_ShutdownNilProviders(t *testing.T) {
	p := &Provider{}
	err := p.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSetup_BuildInfoMetric(t *testing.T) {
	metrics := models.MetricsConfig{
		Enabled: true,
		Path:    "/metrics",
		Port:    9090,
	}
	obs := models.ObservabilityConfig{
		ServiceName: "test-service",
		Tracing:     models.TracingConfig{Enabled: false},
	}
	ver := version.Info{
		Version:   "v9.9.9",
		GitCommit: "abc123",
		BuildDate: "2026-03-07",
	}

	provider, err := Setup(metrics, obs, ver)
	require.NoError(t, err)
	require.NotNil(t, provider)
	defer provider.Shutdown(context.Background())

	// The OTel Prometheus exporter registers itself with prometheus.DefaultRegisterer,
	// so gathering from prometheus.DefaultGatherer will include OTel metrics.
	// Note: other tests in this package also call Setup(), each registering an
	// unchecked OTel collector with the default registry. We search for a sample
	// with our specific label values rather than asserting exactly one sample.
	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var found *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == "updater_build_info" {
			found = mf
			break
		}
	}
	require.NotNil(t, found, "metric family updater_build_info not found in gathered metrics")

	// Find the sample produced by this test's provider (identified by its label values).
	var matched *dto.Metric
	for _, m := range found.GetMetric() {
		labels := make(map[string]string, len(m.GetLabel()))
		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		if labels["version"] == "v9.9.9" && labels["git_commit"] == "abc123" {
			matched = m
			break
		}
	}
	require.NotNil(t, matched, "no updater_build_info sample with version=v9.9.9 git_commit=abc123 found")

	labels := make(map[string]string, len(matched.GetLabel()))
	for _, lp := range matched.GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}
	assert.Equal(t, "v9.9.9", labels["version"])
	assert.Equal(t, "abc123", labels["git_commit"])
	assert.Equal(t, "2026-03-07", labels["build_date"])
	assert.NotEmpty(t, labels["environment"])
	assert.InEpsilon(t, 1.0, matched.GetGauge().GetValue(), 0.001)
}
