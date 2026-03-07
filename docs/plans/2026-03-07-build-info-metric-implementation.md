# Build Info Metric Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `updater_build_info` Prometheus gauge that exposes version metadata (version, git commit, build date, environment) as labels, always returning 1.

**Architecture:** Add a private `registerBuildInfo(ver version.Info) error` method to `Provider` in `internal/observability/observability.go`. `Setup()` calls it after the `MeterProvider` is assigned. The gauge uses the OTel `Int64ObservableGauge` API with a callback that always observes 1. Errors are treated as non-fatal (logged as a warning).

**Tech Stack:** Go, `go.opentelemetry.io/otel/metric`, `go.opentelemetry.io/otel/attribute`, `github.com/prometheus/client_golang/prometheus`, `github.com/stretchr/testify`

---

### Task 1: Write the failing test for `registerBuildInfo`

**Files:**
- Modify: `internal/observability/observability_test.go`

**Step 1: Add the failing test**

Add this test to `internal/observability/observability_test.go`:

```go
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

	// Force the OTel SDK to collect and export to the Prometheus registry.
	// The OTel Prometheus exporter registers itself with prometheus.DefaultRegisterer,
	// so gathering from prometheus.DefaultGatherer will include OTel metrics.
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
	require.Len(t, found.GetMetric(), 1, "expected exactly one sample")

	m := found.GetMetric()[0]
	labels := make(map[string]string, len(m.GetLabel()))
	for _, lp := range m.GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}

	assert.Equal(t, "v9.9.9", labels["version"])
	assert.Equal(t, "abc123", labels["git_commit"])
	assert.Equal(t, "2026-03-07", labels["build_date"])
	assert.NotEmpty(t, labels["environment"])
	assert.InEpsilon(t, 1.0, m.GetGauge().GetValue(), 0.001)
}
```

Add the required imports to the import block (these may already be present):

```go
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
```

**Step 2: Run the test to confirm it fails**

```bash
make test
```

Expected: test fails with `metric family updater_build_info not found in gathered metrics`.

**Step 3: Commit the failing test**

```bash
git add internal/observability/observability_test.go
git commit -m "test: add failing test for updater_build_info gauge"
```

---

### Task 2: Implement `registerBuildInfo`

**Files:**
- Modify: `internal/observability/observability.go`

**Step 1: Add the `registerBuildInfo` method**

Add this method to `observability.go`, after the `setupTracing` function:

```go
// registerBuildInfo registers an updater_build_info gauge that always returns 1
// with version metadata as labels. This follows the standard Prometheus build_info
// pattern, making version data queryable and enabling version-change alerting.
func (p *Provider) registerBuildInfo(ver version.Info) error {
	meter := p.meterProvider.Meter("updater.build")

	gauge, err := meter.Int64ObservableGauge(
		"updater_build_info",
		metric.WithDescription("Build and version information (always 1)."),
		metric.WithUnit("{info}"),
	)
	if err != nil {
		return fmt.Errorf("register build_info gauge: %w", err)
	}

	attrs := metric.WithAttributes(
		attribute.String("version", ver.Version),
		attribute.String("git_commit", ver.GitCommit),
		attribute.String("build_date", ver.BuildDate),
		attribute.String("environment", getEnvironment()),
	)

	_, err = meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			o.ObserveInt64(gauge, 1, attrs)
			return nil
		},
		gauge,
	)
	if err != nil {
		return fmt.Errorf("register build_info callback: %w", err)
	}

	return nil
}
```

**Step 2: Call `registerBuildInfo` from `Setup()`**

In `Setup()`, after `otel.SetMeterProvider(mp)` and before the closing `return p, nil`, add:

```go
	if err := p.registerBuildInfo(ver); err != nil {
		slog.Warn("failed to register build_info metric", "error", err)
	}
```

The full metrics block in `Setup()` should look like:

```go
	// Setup metrics with Prometheus exporter
	if metrics.Enabled {
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		p.promExporter = promExporter

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(promExporter),
		)
		p.meterProvider = mp
		otel.SetMeterProvider(mp)

		if err := p.registerBuildInfo(ver); err != nil {
			slog.Warn("failed to register build_info metric", "error", err)
		}
	}
```

**Step 3: Verify imports**

Ensure `log/slog` is imported in `observability.go` (it likely already is from the rest of the package). The `metric` and `attribute` packages are already imported. No new imports should be needed.

**Step 4: Run the tests**

```bash
make test
```

Expected: `TestSetup_BuildInfoMetric` passes. All other tests continue to pass.

**Step 5: Commit**

```bash
git add internal/observability/observability.go
git commit -m "feat: add updater_build_info Prometheus gauge

Register an Int64ObservableGauge that always returns 1 with labels
version, git_commit, build_date, and environment. Follows the standard
Prometheus build_info pattern — queryable via PromQL and useful for
version-change alerting in Grafana.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Update `docs/observability.md`

**Files:**
- Modify: `docs/observability.md`

**Step 1: Add a "Build Info Metric" subsection**

In `docs/observability.md`, under the **Available Metrics** section and after the **Application Metrics** table, add:

```markdown
#### Build Info Metric

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `updater_build_info` | Gauge | `version`, `git_commit`, `build_date`, `environment` | Always 1; exposes build metadata as labels |

`environment` is read from the `ENVIRONMENT` environment variable, falling back to
`DEPLOYMENT_ENV`, then `development`.

**Example output:**

```
# HELP updater_build_info Build and version information (always 1).
# TYPE updater_build_info gauge
updater_build_info{build_date="2026-03-07",environment="production",git_commit="7fd6828",version="v1.2.3"} 1
```

**Useful PromQL:**

```promql
# How many distinct versions are running? (useful during rolling deploys)
count by (version) (updater_build_info)

# Alert when version changes
changes(updater_build_info[10m]) > 0
```
```

**Step 2: Run tests to confirm nothing is broken**

```bash
make test
```

Expected: all tests pass.

**Step 3: Commit**

```bash
git add docs/observability.md
git commit -m "docs: add build_info metric to observability docs

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Verify and open PR

**Step 1: Run the full test suite**

```bash
make check
```

Expected: format, vet, and all tests pass.

**Step 2: Push the branch**

```bash
git push -u origin feat/build-info-metric
```

**Step 3: Open a PR**

Title: `feat: add updater_build_info Prometheus gauge`

Body:
```
## Summary
- Add `updater_build_info` gauge with `version`, `git_commit`, `build_date`, and `environment` labels
- Follows the standard Prometheus build_info pattern (always returns 1)
- Registered only when metrics are enabled; registration failure is non-fatal
- Updated `docs/observability.md` with metric reference and PromQL examples

## Test plan
- [ ] `TestSetup_BuildInfoMetric` passes and asserts correct label values
- [ ] All existing observability tests continue to pass
- [ ] `make check` passes (fmt + vet + test)
```