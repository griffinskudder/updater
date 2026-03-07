package observability

import (
	"context"
	"testing"
	"updater/internal/models"
	"updater/internal/version"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

	reg := prometheus.NewRegistry()
	provider, err := Setup(metrics, obs, version.Info{}, WithPrometheusRegisterer(reg))
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.NotNil(t, provider.promExporter)
	assert.Nil(t, provider.tracerProvider)

	// updater_build_info is registered as a side effect of Setup when metrics are enabled.
	mfs, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "updater_build_info" {
			found = true
			require.Len(t, mf.GetMetric(), 1)
			assert.InEpsilon(t, 1.0, mf.GetMetric()[0].GetGauge().GetValue(), 0.001)
			break
		}
	}
	assert.True(t, found, "updater_build_info not found in gathered metrics")

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

	reg := prometheus.NewRegistry()
	provider, err := Setup(metrics, obs, version.Info{}, WithPrometheusRegisterer(reg))
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.NotNil(t, provider.tracerProvider)
	assert.NotNil(t, provider.promExporter)

	// updater_build_info is registered as a side effect of Setup when metrics are enabled.
	mfs, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "updater_build_info" {
			found = true
			require.Len(t, mf.GetMetric(), 1)
			assert.InEpsilon(t, 1.0, mf.GetMetric()[0].GetGauge().GetValue(), 0.001)
			break
		}
	}
	assert.True(t, found, "updater_build_info not found in gathered metrics")

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

	// Use an isolated registry so this test does not touch prometheus.DefaultRegisterer.
	reg := prometheus.NewRegistry()
	provider, err := Setup(metrics, obs, ver, WithPrometheusRegisterer(reg))
	require.NoError(t, err)
	require.NotNil(t, provider)
	defer provider.Shutdown(context.Background())

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var found *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == "updater_build_info" {
			found = mf
			break
		}
	}
	require.NotNil(t, found, "metric family updater_build_info not found in gathered metrics")
	require.Len(t, found.GetMetric(), 1, "expected exactly one updater_build_info sample")

	labels := make(map[string]string, len(found.GetMetric()[0].GetLabel()))
	for _, lp := range found.GetMetric()[0].GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}
	assert.Equal(t, "v9.9.9", labels["version"])
	assert.Equal(t, "abc123", labels["git_commit"])
	assert.Equal(t, "2026-03-07", labels["build_date"])
	// getEnvironment() falls back to "development" when no env vars are set.
	assert.Equal(t, "development", labels["environment"])
	assert.InEpsilon(t, 1.0, found.GetMetric()[0].GetGauge().GetValue(), 0.001)
}
