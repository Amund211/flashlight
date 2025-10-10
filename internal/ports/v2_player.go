package ports

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

func MakeGetV2PlayerHandler(
	getAndPersistPlayerWithCache app.GetAndPersistPlayerWithCache,
	allowedOrigins *DomainSuffixes,
	rootLogger *slog.Logger,
	sentryMiddleware func(http.HandlerFunc) http.HandlerFunc,
) http.HandlerFunc {
	ipLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(8),
		ratelimiting.BurstSize(480),
	)
	ipRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		ipLimiter,
		ratelimiting.IPKeyFunc,
	)
	userIDLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(2),
		ratelimiting.BurstSize(120),
	)
	userIDRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		// NOTE: Rate limiting based on user controlled value
		userIDLimiter,
		ratelimiting.UserIDKeyFunc,
	)

	makeOnLimitExceeded := func(rateLimiter ratelimiting.RequestRateLimiter) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger := logging.FromContext(ctx)

			statusCode := http.StatusTooManyRequests
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			
			errorData, err := PlayerToV2PlayerErrorResponseData("Rate limit exceeded")
			if err != nil {
				w.Write([]byte(`{"success":false,"cause":"Rate limit exceeded"}`))
			} else {
				w.Write(errorData)
			}

			logger.Info("Returning response", "statusCode", statusCode, "reason", "ratelimit exceeded", "key", rateLimiter.KeyFor(r))
		}
	}

	middleware := ComposeMiddlewares(
		logging.NewRequestLoggerMiddleware(rootLogger),
		sentryMiddleware,
		reporting.NewAddMetaMiddleware("v2-player"),
		BuildCORSMiddleware(allowedOrigins),
		NewRateLimitMiddleware(ipRateLimiter, makeOnLimitExceeded(ipRateLimiter)),
		NewRateLimitMiddleware(userIDRateLimiter, makeOnLimitExceeded(userIDRateLimiter)),
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		rawUUID := r.PathValue("uuid")
		userID := r.Header.Get("X-User-Id")
		ctx = reporting.SetUserIDInContext(ctx, userID)
		if userID == "" {
			userID = "<missing>"
		}
		ctx = logging.AddMetaToContext(ctx,
			slog.String("userId", userID),
			slog.String("uuid", rawUUID),
		)
		logger := logging.FromContext(ctx)

		ctx = reporting.AddExtrasToContext(ctx,
			map[string]string{
				"rawUUID": rawUUID,
			},
		)

		uuid, err := strutils.NormalizeUUID(rawUUID)
		if err != nil {
			statusCode := http.StatusBadRequest

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			
			errorData, marshalErr := PlayerToV2PlayerErrorResponseData("Invalid UUID")
			if marshalErr != nil {
				w.Write([]byte(`{"success":false,"cause":"Invalid UUID"}`))
			} else {
				w.Write(errorData)
			}

			logger.Info("Returning response", "statusCode", statusCode, "reason", "invalid uuid")
			return
		}

		ctx = reporting.AddExtrasToContext(ctx,
			map[string]string{
				"uuid": uuid,
			},
		)

		player, err := getAndPersistPlayerWithCache(ctx, uuid)
		if errors.Is(err, domain.ErrPlayerNotFound) {
			v2ResponseData, err := PlayerToV2PlayerResponseData(nil)
			if err != nil {
				logger.Error("Failed to convert player to V2 response", "error", err)
				err = fmt.Errorf("failed to convert player to V2 response: %w", err)
				reporting.Report(ctx, err)
				statusCode := writeV2ErrorResponse(ctx, w, err)
				logger.Info("Returning response", "statusCode", statusCode, "reason", "error")
				return
			}

			statusCode := 404
			logger.Info("Returning response", "statusCode", statusCode, "reason", "player not found")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write(v2ResponseData)
			return
		}

		if err != nil {
			// NOTE: GetAndPersistPlayerWithCache implementations handle their own error reporting
			logger.Error("Error getting player data", "error", err)
			statusCode := writeV2ErrorResponse(ctx, w, err)
			logger.Info("Returning response", "statusCode", statusCode, "reason", "error")
			return
		}

		v2ResponseData, err := PlayerToV2PlayerResponseData(player)
		if err != nil {
			logger.Error("Failed to convert player to V2 response", "error", err)

			err = fmt.Errorf("failed to convert player to V2 response: %w", err)
			reporting.Report(ctx, err)

			statusCode := writeV2ErrorResponse(ctx, w, err)
			logger.Info("Returning response", "statusCode", statusCode, "reason", "error")
			return
		}

		logger.Info("Got V2 player data", "contentLength", len(v2ResponseData), "statusCode", 200)

		statusCode := 200
		logger.Info("Returning response", "statusCode", statusCode, "reason", "success")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(v2ResponseData)
	}

	return middleware(handler)
}

func writeV2ErrorResponse(ctx context.Context, w http.ResponseWriter, responseError error) int {
	w.Header().Set("Content-Type", "application/json")

	// Unknown error: default to 500
	statusCode := http.StatusInternalServerError
	cause := "Internal server error"

	if errors.Is(responseError, domain.ErrTemporarilyUnavailable) {
		statusCode = http.StatusServiceUnavailable
		cause = "Service temporarily unavailable"
	}

	errorData, err := PlayerToV2PlayerErrorResponseData(cause)
	if err != nil {
		logging.FromContext(ctx).Error("Failed to marshal V2 error response", "error", err)
		reporting.Report(ctx, fmt.Errorf("failed to marshal V2 error response: %w", err), map[string]string{
			"responseError": responseError.Error(),
		})
		w.WriteHeader(statusCode)
		w.Write([]byte(`{"success":false,"cause":"Internal server error"}`))
		return statusCode
	}

	w.WriteHeader(statusCode)
	w.Write(errorData)

	return statusCode
}