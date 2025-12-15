package ports

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
)

type searchUsernameResponseObject struct {
	UUIDs []string `json:"uuids"`
}

func MakeSearchUsernameHandler(
	searchUsername app.SearchUsername,
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

			statusCode := http.StatusTooManyRequests

			logging.FromContext(ctx).InfoContext(ctx, "Rate limit exceeded", "statusCode", statusCode, "reason", "ratelimit exceeded", "key", rateLimiter.KeyFor(r))

			http.Error(w, "Rate limit exceeded", statusCode)
		}
	}

	middleware := ComposeMiddlewares(
		buildMetricsMiddleware("search_username"),
		logging.NewRequestLoggerMiddleware(rootLogger),
		sentryMiddleware,
		reporting.NewAddMetaMiddleware("search_username"),
		NewRateLimitMiddleware(ipRateLimiter, makeOnLimitExceeded(ipRateLimiter)),
		NewRateLimitMiddleware(userIDRateLimiter, makeOnLimitExceeded(userIDRateLimiter)),
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		searchTerm := r.URL.Query().Get("q")
		topStr := r.URL.Query().Get("top")

		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			userID = "<missing>"
		}
		ctx = reporting.SetUserIDInContext(ctx, userID)
		ctx = logging.AddMetaToContext(ctx,
			slog.String("userId", userID),
			slog.String("searchTerm", searchTerm),
			slog.String("top", topStr),
		)
		ctx = reporting.AddExtrasToContext(ctx,
			map[string]string{
				"searchTerm": searchTerm,
				"top":        topStr,
			},
		)

		if searchTerm == "" {
			statusCode := http.StatusBadRequest
			logging.FromContext(ctx).InfoContext(ctx, "Missing search term", "statusCode", statusCode, "reason", "missing search term")
			http.Error(w, "Missing search term parameter 'q'", statusCode)
			return
		}

		top := 10 // default
		if topStr != "" {
			var err error
			top, err = strconv.Atoi(topStr)
			if err != nil || top < 1 || top > 100 {
				statusCode := http.StatusBadRequest
				logging.FromContext(ctx).InfoContext(ctx, "Invalid top parameter", "statusCode", statusCode, "reason", "invalid top parameter")
				http.Error(w, "Invalid top parameter, must be between 1 and 100", statusCode)
				return
			}
		}

		uuids, err := searchUsername(ctx, searchTerm, top)
		if err != nil {
			logging.FromContext(ctx).ErrorContext(ctx, "Error searching username", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		responseData, err := json.Marshal(searchUsernameResponseObject{UUIDs: uuids})
		if err != nil {
			logging.FromContext(ctx).ErrorContext(ctx, "Failed to marshal response", "error", err)

			err = fmt.Errorf("failed to marshal search username response: %w", err)
			reporting.Report(ctx, err)

			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(responseData); err != nil {
			logging.FromContext(ctx).ErrorContext(ctx, "Failed to write response", "error", err)
			reporting.Report(ctx, fmt.Errorf("failed to write search username response: %w", err))

			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	return middleware(handler)
}
