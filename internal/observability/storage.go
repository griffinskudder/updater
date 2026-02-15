package observability

import (
	"context"
	"time"
	"updater/internal/models"
	"updater/internal/storage"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedStorage wraps a storage.Storage implementation with
// OpenTelemetry tracing and metrics instrumentation.
type InstrumentedStorage struct {
	inner    storage.Storage
	tracer   trace.Tracer
	duration metric.Float64Histogram
	errors   metric.Int64Counter
}

// NewInstrumentedStorage creates a new storage wrapper that records trace spans,
// operation latency histograms, and error counters for every storage method call.
func NewInstrumentedStorage(inner storage.Storage) (*InstrumentedStorage, error) {
	tracer := otel.Tracer("updater/storage")
	meter := otel.Meter("updater/storage")

	duration, err := meter.Float64Histogram(
		"storage.operation.duration",
		metric.WithDescription("Duration of storage operations in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	errCounter, err := meter.Int64Counter(
		"storage.operation.errors",
		metric.WithDescription("Number of storage operation errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return &InstrumentedStorage{
		inner:    inner,
		tracer:   tracer,
		duration: duration,
		errors:   errCounter,
	}, nil
}

func (s *InstrumentedStorage) startSpan(ctx context.Context, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := s.tracer.Start(ctx, "storage."+operation,
		trace.WithAttributes(append([]attribute.KeyValue{
			attribute.String("storage.operation", operation),
		}, attrs...)...),
	)
	return ctx, span
}

func (s *InstrumentedStorage) record(ctx context.Context, span trace.Span, operation string, start time.Time, err error) {
	elapsed := time.Since(start).Seconds()
	attrs := metric.WithAttributes(attribute.String("operation", operation))

	s.duration.Record(ctx, elapsed, attrs)

	if err != nil {
		s.errors.Add(ctx, 1, attrs)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.End()
}

func (s *InstrumentedStorage) Applications(ctx context.Context) ([]*models.Application, error) {
	ctx, span := s.startSpan(ctx, "Applications")
	start := time.Now()
	result, err := s.inner.Applications(ctx)
	s.record(ctx, span, "Applications", start, err)
	return result, err
}

func (s *InstrumentedStorage) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	ctx, span := s.startSpan(ctx, "GetApplication", attribute.String("app_id", appID))
	start := time.Now()
	result, err := s.inner.GetApplication(ctx, appID)
	s.record(ctx, span, "GetApplication", start, err)
	return result, err
}

func (s *InstrumentedStorage) SaveApplication(ctx context.Context, app *models.Application) error {
	ctx, span := s.startSpan(ctx, "SaveApplication", attribute.String("app_id", app.ID))
	start := time.Now()
	err := s.inner.SaveApplication(ctx, app)
	s.record(ctx, span, "SaveApplication", start, err)
	return err
}

func (s *InstrumentedStorage) DeleteApplication(ctx context.Context, appID string) error {
	ctx, span := s.startSpan(ctx, "DeleteApplication",
		attribute.String("app_id", appID),
	)
	start := time.Now()
	err := s.inner.DeleteApplication(ctx, appID)
	s.record(ctx, span, "DeleteApplication", start, err)
	return err
}

func (s *InstrumentedStorage) Releases(ctx context.Context, appID string) ([]*models.Release, error) {
	ctx, span := s.startSpan(ctx, "Releases", attribute.String("app_id", appID))
	start := time.Now()
	result, err := s.inner.Releases(ctx, appID)
	s.record(ctx, span, "Releases", start, err)
	return result, err
}

func (s *InstrumentedStorage) GetRelease(ctx context.Context, appID, version, platform, arch string) (*models.Release, error) {
	ctx, span := s.startSpan(ctx, "GetRelease",
		attribute.String("app_id", appID),
		attribute.String("version", version),
		attribute.String("platform", platform),
		attribute.String("arch", arch),
	)
	start := time.Now()
	result, err := s.inner.GetRelease(ctx, appID, version, platform, arch)
	s.record(ctx, span, "GetRelease", start, err)
	return result, err
}

func (s *InstrumentedStorage) SaveRelease(ctx context.Context, release *models.Release) error {
	ctx, span := s.startSpan(ctx, "SaveRelease",
		attribute.String("app_id", release.ApplicationID),
		attribute.String("version", release.Version),
		attribute.String("platform", release.Platform),
		attribute.String("arch", release.Architecture),
	)
	start := time.Now()
	err := s.inner.SaveRelease(ctx, release)
	s.record(ctx, span, "SaveRelease", start, err)
	return err
}

func (s *InstrumentedStorage) DeleteRelease(ctx context.Context, appID, version, platform, arch string) error {
	ctx, span := s.startSpan(ctx, "DeleteRelease",
		attribute.String("app_id", appID),
		attribute.String("version", version),
		attribute.String("platform", platform),
		attribute.String("arch", arch),
	)
	start := time.Now()
	err := s.inner.DeleteRelease(ctx, appID, version, platform, arch)
	s.record(ctx, span, "DeleteRelease", start, err)
	return err
}

func (s *InstrumentedStorage) GetLatestRelease(ctx context.Context, appID, platform, arch string) (*models.Release, error) {
	ctx, span := s.startSpan(ctx, "GetLatestRelease",
		attribute.String("app_id", appID),
		attribute.String("platform", platform),
		attribute.String("arch", arch),
	)
	start := time.Now()
	result, err := s.inner.GetLatestRelease(ctx, appID, platform, arch)
	s.record(ctx, span, "GetLatestRelease", start, err)
	return result, err
}

func (s *InstrumentedStorage) GetReleasesAfterVersion(ctx context.Context, appID, currentVersion, platform, arch string) ([]*models.Release, error) {
	ctx, span := s.startSpan(ctx, "GetReleasesAfterVersion",
		attribute.String("app_id", appID),
		attribute.String("current_version", currentVersion),
		attribute.String("platform", platform),
		attribute.String("arch", arch),
	)
	start := time.Now()
	result, err := s.inner.GetReleasesAfterVersion(ctx, appID, currentVersion, platform, arch)
	s.record(ctx, span, "GetReleasesAfterVersion", start, err)
	return result, err
}

func (s *InstrumentedStorage) Ping(ctx context.Context) error {
	ctx, span := s.startSpan(ctx, "Ping")
	start := time.Now()
	err := s.inner.Ping(ctx)
	s.record(ctx, span, "Ping", start, err)
	return err
}

func (s *InstrumentedStorage) Close() error {
	return s.inner.Close()
}
