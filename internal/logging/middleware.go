package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"
)

type requestLoggerContextKey struct{}

func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(requestLoggerContextKey{}).(*slog.Logger)
	if !ok || logger == nil {
		fallback := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		fallback = fallback.With(slog.String("logger", "fallback"))
		return fallback
	}
	return logger
}

func NewRequestLoggerMiddleware(logger *slog.Logger) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			correlationID := uuid.New().String()

			uuid := r.URL.Query().Get("uuid")
			if uuid == "" {
				uuid = "<missing>"
			}

			userId := r.Header.Get("X-User-Id")
			if userId == "" {
				userId = "<missing>"
			}

			userAgent := r.UserAgent()
			if userAgent == "" {
				userAgent = "<missing>"
			}

			requestLogger := logger.With(
				slog.String("correlationID", correlationID),
				slog.String("uuid", uuid),
				slog.String("userId", userId),
				slog.String("userAgent", userAgent),
				slog.String("methodPath", fmt.Sprintf("%s %s", r.Method, r.URL.Path)),
			)

			next(w, r.WithContext(context.WithValue(r.Context(), requestLoggerContextKey{}, requestLogger)))
		}
	}
}
