package logging

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type requestLoggerContextKey struct{}

func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(requestLoggerContextKey{}).(*slog.Logger)
	if !ok || logger == nil {
		fallback := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		fallback = fallback.With(slog.String("logger", "fallback"))
		logger = fallback
	}

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		// Associate the logs in google cloud with the active trace/span.
		// https://docs.cloud.google.com/logging/docs/agent/logging/configuration#special-fields
		logger = logger.With(
			slog.String("logging.googleapis.com/trace", sc.TraceID().String()),
			slog.String("logging.googleapis.com/spanId", sc.SpanID().String()),
			slog.Bool("logging.googleapis.com/trace_sampled", sc.TraceFlags().IsSampled()),
		)
	}

	return logger
}

func AddToContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, requestLoggerContextKey{}, logger)
}

func AddMetaToContext(ctx context.Context, args ...slog.Attr) context.Context {
	logger := FromContext(ctx)

	// Convert our []slog.Attr to []any
	anySlice := make([]any, len(args))
	for i, arg := range args {
		anySlice[i] = arg
	}

	withMeta := logger.With(anySlice...)

	return AddToContext(ctx, withMeta)
}
