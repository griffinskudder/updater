# Future Enhancements

This document tracks potential enhancements and improvements that could be made to the updater service. These are not currently planned for implementation but represent opportunities for future development.

## Version Metadata Enhancements

Building on the version metadata feature implemented in February 2026, the following enhancements could further improve observability and user experience.

### 1. Admin UI Version Display

**Status:** Not Implemented
**Priority:** Low
**Effort:** Small (1-2 hours)

**Description:**

Add build metadata display to the admin UI dashboard and footer.

**Benefits:**
- Administrators can quickly verify which version is deployed
- Helpful for troubleshooting and deployment verification
- Provides deployment environment visibility

**Implementation:**

Add to admin dashboard (`/admin`):
```html
<div class="version-info">
  <strong>Version:</strong> {{.Version}}
  <span class="commit">({{.GitCommit}})</span>
  <span class="build-date">Built: {{.BuildDate}}</span>
  <span class="environment">Environment: {{.Environment}}</span>
</div>
```

Add to admin footer (all admin pages):
```html
<footer>
  <p>Updater Service v{{.Version}} | Instance: {{.InstanceID}}</p>
</footer>
```

**Files to modify:**
- `internal/api/handlers.go` - Pass version info to admin templates
- `web/templates/admin/*.html` - Add version display elements
- `web/static/css/admin.css` - Style version display

**Considerations:**
- Keep styling minimal and unobtrusive
- Consider making it a collapsible info panel
- Environment badge could use color coding (dev=blue, staging=yellow, production=green)

---

### 2. Error Response Metadata

**Status:** Not Implemented
**Priority:** Low
**Effort:** Small (1-2 hours)

**Description:**

Include build metadata in error responses to help with debugging and support.

**Benefits:**
- Support teams can quickly identify which version produced an error
- Easier correlation between error reports and deployed versions
- Helpful for diagnosing version-specific issues

**Current error response:**
```json
{
  "error": "application not found",
  "details": "application 'myapp' does not exist",
  "status": 404
}
```

**Enhanced error response:**
```json
{
  "error": "application not found",
  "details": "application 'myapp' does not exist",
  "status": 404,
  "metadata": {
    "version": "v1.0.0",
    "instance_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2026-02-21T10:30:00Z"
  }
}
```

**Files to modify:**
- `internal/models/response.go` - Add ErrorResponse metadata field
- `internal/api/handlers.go` - Include version info in error responses
- `internal/api/openapi/openapi.yaml` - Update error response schema

**Considerations:**
- Only include in error responses, not in success responses (avoid payload bloat)
- Keep metadata minimal (version, instance_id, timestamp)
- Document in OpenAPI spec
- Consider making this optional via configuration flag

---

### 3. Version Comparison Endpoint

**Status:** Not Implemented
**Priority:** Low
**Effort:** Medium (3-4 hours)

**Description:**

Add a `/version/compare` endpoint that compares the service version with a provided version string.

**Benefits:**
- Client applications can verify server compatibility
- Useful for multi-version API support
- Helps with rolling deployments and gradual rollouts

**Example usage:**
```bash
curl http://localhost:8080/version/compare?version=v1.0.0
```

**Response:**
```json
{
  "server_version": "v1.2.0",
  "client_version": "v1.0.0",
  "compatible": true,
  "comparison": "newer",
  "details": "Server is 2 minor versions ahead"
}
```

**Implementation:**

New endpoint in `internal/api/handlers.go`:
```go
func (h *Handlers) CompareVersion(w http.ResponseWriter, r *http.Request) {
    clientVersion := r.URL.Query().Get("version")
    if clientVersion == "" {
        h.writeErrorResponse(w, http.StatusBadRequest, "version parameter required")
        return
    }

    result := version.Compare(h.versionInfo.Version, clientVersion)
    h.writeJSONResponse(w, http.StatusOK, result)
}
```

**Files to modify:**
- `internal/version/version.go` - Add Compare() function using semver
- `internal/api/handlers.go` - Add CompareVersion handler
- `internal/api/routes.go` - Register `/version/compare` route
- `internal/api/openapi/openapi.yaml` - Document endpoint

**Considerations:**
- Use semantic versioning rules for comparison
- Handle non-semver versions gracefully
- Define compatibility rules (e.g., major version must match)
- Consider rate limiting this endpoint

---

### 4. Deployment History Tracking

**Status:** Not Implemented
**Priority:** Low
**Effort:** Large (2-3 days)

**Description:**

Track deployment history by persisting version information to storage on startup, creating an audit trail of deployments.

**Benefits:**
- Historical record of what versions were deployed and when
- Helps with deployment auditing and compliance
- Useful for rollback planning and incident investigation

**Data model:**
```go
type DeploymentRecord struct {
    ID          string    `json:"id"`
    Version     string    `json:"version"`
    GitCommit   string    `json:"git_commit"`
    BuildDate   string    `json:"build_date"`
    InstanceID  string    `json:"instance_id"`
    Hostname    string    `json:"hostname"`
    Environment string    `json:"environment"`
    StartTime   time.Time `json:"start_time"`
    EndTime     *time.Time `json:"end_time,omitempty"`
}
```

**New endpoints:**
- `GET /api/v1/admin/deployments` - List deployment history
- `GET /api/v1/admin/deployments/current` - Show currently running instances

**Implementation considerations:**
- Add deployment_history table to database schema
- Record deployment on service startup
- Mark deployment as ended on graceful shutdown
- Handle multiple concurrent instances (Kubernetes deployments)
- Add retention policy (e.g., keep last 100 deployments or 90 days)
- Require admin permission for access

**Files to modify:**
- `internal/storage/schema/*/002_deployment_history.sql` - New migration
- `internal/storage/queries/*/deployments.sql` - CRUD queries
- `internal/models/deployment.go` - DeploymentRecord model
- `cmd/updater/updater.go` - Record deployment on startup
- `internal/api/handlers.go` - Deployment history endpoints
- `internal/api/routes.go` - Register new routes
- `internal/api/openapi/openapi.yaml` - Document endpoints

**Storage considerations:**
- Not needed for memory or JSON storage (ephemeral)
- PostgreSQL and SQLite only
- Add index on start_time for efficient queries

---

### 5. Build Metadata in Metrics

**Status:** Not Implemented
**Priority:** Low
**Effort:** Small (1 hour)

**Description:**

Add a Prometheus `build_info` metric with version metadata as labels.

**Benefits:**
- Version information queryable via Prometheus
- Correlate metrics with specific versions
- Alert on version changes or mismatches in a cluster

**Implementation:**

Add to `internal/observability/observability.go`:
```go
func (p *Provider) registerBuildInfo(ver version.Info) {
    meter := p.meterProvider.Meter("updater")

    buildInfo, _ := meter.Int64ObservableGauge(
        "build_info",
        metric.WithDescription("Build and version information"),
    )

    _, _ = meter.RegisterCallback(
        func(ctx context.Context, o metric.Observer) error {
            o.ObserveInt64(buildInfo, 1,
                metric.WithAttributes(
                    attribute.String("version", ver.Version),
                    attribute.String("git_commit", ver.GitCommit),
                    attribute.String("build_date", ver.BuildDate),
                ))
            return nil
        },
        buildInfo,
    )
}
```

**Prometheus query example:**
```promql
build_info{version="v1.0.0"}
```

**Files to modify:**
- `internal/observability/observability.go` - Add registerBuildInfo() call

**Considerations:**
- This is a gauge that always returns 1 with labels
- Standard pattern used by many projects (Kubernetes, Prometheus itself)
- Useful for Grafana dashboards showing deployed versions

---

## Implementation Priority

If implementing these enhancements, the recommended order is:

1. **Build Metadata in Metrics** (1 hour) - Highest value, lowest effort
2. **Admin UI Version Display** (1-2 hours) - Visible, useful for operators
3. **Error Response Metadata** (2 hours) - Helpful for debugging
4. **Version Comparison Endpoint** (3-4 hours) - Useful for client compatibility
5. **Deployment History Tracking** (2-3 days) - Most complex, consider if needed

## Related Documentation

- [ARCHITECTURE.md](../ARCHITECTURE.md#7-version-metadata-internalversion-complete) - Version package architecture
- [observability.md](../observability.md#resource-attributes) - Resource attributes and build metadata
- [logging.md](../logging.md#global-fields) - Global log fields
