# Design: `build_info` Prometheus Gauge

**Date:** 2026-03-07
**Status:** Approved
**Effort:** Small (1-2 hours)

## Background

The observability package exposes HTTP request metrics and business metrics
(update checks, release registrations) via the OTel Prometheus exporter.
Version metadata (version string, git commit, build date, environment) is
already attached to the OTel resource and appears in `target_info`, but it is
not queryable as a standalone metric.

The `build_info` pattern is standard across the Prometheus ecosystem
(Prometheus itself, Kubernetes, VictoriaMetrics, etc.): a gauge that always
returns 1 with version metadata as labels, making it trivial to join version
information onto other metrics and alert on version changes in a cluster.

## Goal

Add `updater_build_info` — an `Int64ObservableGauge` that always returns 1
with labels carrying the build and runtime metadata already available in
`version.Info`.

## Design

### Metric definition

| Property | Value |
|----------|-------|
| Name | `updater_build_info` |
| Type | `Int64ObservableGauge` (OTel) / `gauge` (Prometheus) |
| Value | Always `1` |
| Labels | `version`, `git_commit`, `build_date`, `environment` |
| Instrument scope | `updater.build` |

Labels:

- `version` — from `version.Info.Version` (e.g. `v1.2.3` or `unknown`)
- `git_commit` — from `version.Info.GitCommit`
- `build_date` — from `version.Info.BuildDate`
- `environment` — from `getEnvironment()` (reads `ENVIRONMENT` or
  `DEPLOYMENT_ENV` env var, defaults to `development`)

`instance_id` and `hostname` are omitted from `build_info` labels because
they are high-cardinality per-instance values already present on the OTel
resource (and thus in `target_info`). Putting them on `build_info` would
create a new time series per instance restart.

### Implementation

Add a private `registerBuildInfo(ver version.Info) error` method to
`Provider`. `Setup()` calls it immediately after `p.meterProvider` is
assigned:

```go
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
    return err
}
```

`Setup()` propagates the error:

```go
if err := p.registerBuildInfo(ver); err != nil {
    slog.Warn("failed to register build_info metric", "error", err)
    // non-fatal: continue without build_info
}
```

The error is treated as non-fatal (logged as a warning) because a missing
`build_info` gauge should not prevent the service from starting.

### Files changed

| File | Change |
|------|--------|
| `internal/observability/observability.go` | Add `registerBuildInfo`, call from `Setup()` |
| `internal/observability/observability_test.go` | Add test asserting label values |
| `docs/observability.md` | Add "Build Info Metric" section with PromQL example |

## Testing

One test case added to `observability_test.go`:

1. Call `Setup()` with a known `version.Info{Version: "v9.9.9", GitCommit: "abc123", BuildDate: "2026-03-07"}`.
2. Collect metrics from the Prometheus registry.
3. Assert a metric family named `updater_build_info` exists.
4. Assert the single sample has value `1`.
5. Assert label values match the input `version.Info`.

## Example Prometheus output

```
# HELP updater_build_info Build and version information (always 1).
# TYPE updater_build_info gauge
updater_build_info{build_date="2026-03-07",environment="production",git_commit="7fd6828",version="v1.2.3"} 1
```

## Example PromQL

```promql
# Is more than one version running? (useful in rolling deployments)
count by (version) (updater_build_info)

# Alert when version changes
changes(updater_build_info[10m]) > 0
```