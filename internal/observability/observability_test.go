package observability

import (
	"context"
	"testing"
	"updater/internal/models"
	"updater/internal/version"

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
