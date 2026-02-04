package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
	Enabled       bool
	ServiceName   string
	Endpoint      string
	Protocol      string // "grpc" or "http"
	Insecure      bool
	EnableTraces  bool
	EnableMetrics bool
	EnableLogs    bool
}

type Provider struct {
	traceProvider  *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	resource       *resource.Resource
}

func Setup(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		return &Provider{}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("dev"),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	provider := &Provider{
		resource: res,
	}

	// Setup trace provider
	if cfg.EnableTraces {
		tp, err := setupTraceProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("failed to setup trace provider: %w", err)
		}
		provider.traceProvider = tp
		otel.SetTracerProvider(tp)
	}

	// Setup meter provider
	if cfg.EnableMetrics {
		mp, err := setupMeterProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("failed to setup meter provider: %w", err)
		}
		provider.meterProvider = mp
		otel.SetMeterProvider(mp)
	}

	// Setup logger provider
	if cfg.EnableLogs {
		lp, err := setupLoggerProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("failed to setup logger provider: %w", err)
		}
		provider.loggerProvider = lp
		global.SetLoggerProvider(lp)
	}

	// Setup propagators
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

func setupTraceProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	if cfg.Protocol == "grpc" {
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
	} else {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
	}

	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	return tp, nil
}

func setupMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	var exporter sdkmetric.Exporter
	var err error

	if cfg.Protocol == "grpc" {
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		exporter, err = otlpmetricgrpc.New(ctx, opts...)
	} else {
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		exporter, err = otlpmetrichttp.New(ctx, opts...)
	}

	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)

	return mp, nil
}

func setupLoggerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	var exporter sdklog.Exporter
	var err error

	if cfg.Protocol == "grpc" {
		opts := []otlploggrpc.Option{
			otlploggrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		exporter, err = otlploggrpc.New(ctx, opts...)
	} else {
		opts := []otlploghttp.Option{
			otlploghttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		exporter, err = otlploghttp.New(ctx, opts...)
	}

	if err != nil {
		return nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	return lp, nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	if p.traceProvider != nil {
		if err := p.traceProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
		}
	}

	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	if p.loggerProvider != nil {
		if err := p.loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("logger provider shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}
