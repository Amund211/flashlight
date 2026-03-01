package ports

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func IPKeyFunc(r *http.Request) string {
	return fmt.Sprintf("ip: %s", GetIP(r))
}

func UserIDKeyFunc(r *http.Request) string {
	return fmt.Sprintf("user-id: %s", GetUserID(r))
}

func NewRateLimitMiddleware(rateLimiter ratelimiting.RequestRateLimiter, onLimitExceeded http.HandlerFunc) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !rateLimiter.Consume(r) {
				onLimitExceeded(w, r)
				return
			}

			next(w, r)
		}
	}
}

func BuildRegisterUserVisitMiddleware(registerUserVisit app.RegisterUserVisit) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			go func() {
				// NOTE: Since we're doing this in a goroutine, we want a context
				//       that won't get cancelled when the request ends
				ctx, cancel := context.WithTimeout(
					context.WithoutCancel(r.Context()),
					1*time.Second,
				)
				defer cancel()

				userID := GetUserID(r)

				_, _ = registerUserVisit(ctx, userID)
			}()

			next(w, r)
		}
	}
}

type BlocklistConfig struct {
	IPs        []string
	UserAgents []string
	UserIDs    []string
}

func BuildBlocklistMiddleware(config BlocklistConfig, logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ip := GetIP(r)
			userAgent := r.UserAgent()
			userID := GetUserID(r)

			var blockReason string
			if slices.Contains(config.IPs, ip) {
				blockReason = "ip"
			} else if slices.Contains(config.UserAgents, userAgent) {
				blockReason = "user_agent"
			} else if slices.Contains(config.UserIDs, userID) {
				blockReason = "user_id"
			}

			if blockReason != "" {
				// Log the blocked request with details
				requestLogger := logger
				if ctxLogger := logging.FromContext(ctx); ctxLogger != nil {
					requestLogger = ctxLogger
				}
				requestLogger.InfoContext(ctx, "Blocked request",
					slog.String("ip", ip),
					slog.String("userAgent", userAgent),
					slog.String("userID", userID),
					slog.String("blockReason", blockReason),
				)

				// Record metric with block reason (not including high-cardinality labels)
				attributes := []attribute.KeyValue{
					attribute.String("block_reason", blockReason),
				}
				metrics.blockedRequestCount.Add(ctx, 1, metric.WithAttributes(attributes...))

				http.Error(w, `{"success": false, "detail": "This API does not allow third-party use. Reach out on the Prism discord if you have questions :^) (https://discord.gg/k4FGUnEHYg)"}`, http.StatusBadRequest)
				return
			}
			next(w, r)
		}
	}
}

func ComposeMiddlewares(middlewares ...func(http.HandlerFunc) http.HandlerFunc) func(http.HandlerFunc) http.HandlerFunc {
	if len(middlewares) == 1 {
		return middlewares[0]
	}
	first := middlewares[0]
	rest := ComposeMiddlewares(middlewares[1:]...)
	return func(h http.HandlerFunc) http.HandlerFunc {
		return first(rest(h))
	}
}
