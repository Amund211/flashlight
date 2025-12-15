package ports

import (
	"context"
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

type searchUsernameResponse struct {
	Success bool     `json:"success"`
	UUIDs   []string `json:"uuids"`
	Cause   string   `json:"cause,omitempty"`
}

func MakeSearchUsernameHandler(
	searchUsername app.SearchUsername,
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
		userIDLimiter,
		ratelimiting.UserIDKeyFunc,
	)

	makeOnLimitExceeded := func(rateLimiter ratelimiting.RequestRateLimiter) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"cause":"rate limit exceeded"}`))
		}
	}

	middleware := ComposeMiddlewares(
		buildMetricsMiddleware("search_username"),
		logging.NewRequestLoggerMiddleware(rootLogger),
		sentryMiddleware,
		reporting.NewAddMetaMiddleware("search_username"),
		BuildCORSMiddleware(allowedOrigins),
		NewRateLimitMiddleware(ipRateLimiter, makeOnLimitExceeded(ipRateLimiter)),
		NewRateLimitMiddleware(userIDRateLimiter, makeOnLimitExceeded(userIDRateLimiter)),
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		search := r.URL.Query().Get("search")
		topStr := r.URL.Query().Get("top")

		handleError := func(ctx context.Context, cause string, statusCode int) {
			response, err := makeSearchUsernameErrorResponse(ctx, cause)
			if err != nil {
				reporting.Report(ctx, fmt.Errorf("failed to marshal error response: %w", err))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"success":false,"cause":"internal server error"}`))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write(response)
		}

		userID := r.Header.Get("X-User-Id")
		ctx = reporting.SetUserIDInContext(ctx, userID)
		if userID == "" {
			userID = "<missing>"
		}
		ctx = logging.AddMetaToContext(ctx,
			slog.String("userId", userID),
			slog.String("search", search),
			slog.String("top", topStr),
		)
		ctx = reporting.AddExtrasToContext(ctx,
			map[string]string{
				"search": search,
				"top":    topStr,
			},
		)

		// Validate search parameter
		searchLength := len(search)
		if searchLength == 0 || searchLength > 100 {
			handleError(ctx, "invalid search length", http.StatusBadRequest)
			return
		}

		// Parse and validate top parameter
		top := 10 // Default value
		if topStr != "" {
			var err error
			top, err = strconv.Atoi(topStr)
			if err != nil || top < 1 || top > 100 {
				handleError(ctx, "invalid top parameter", http.StatusBadRequest)
				return
			}
		}

		uuids, err := searchUsername(ctx, search, top)
		if err != nil {
			// NOTE: searchUsername implementations handle their own error reporting
			handleError(ctx, "internal server error", http.StatusInternalServerError)
			return
		}

		response, err := makeSearchUsernameSuccessResponse(ctx, uuids)
		if err != nil {
			reporting.Report(ctx, fmt.Errorf("failed to create success response: %w", err))
			handleError(ctx, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)
	}

	return middleware(handler)
}

func makeSearchUsernameSuccessResponse(ctx context.Context, uuids []string) ([]byte, error) {
	resp := searchUsernameResponse{
		Success: true,
		UUIDs:   uuids,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return data, nil
}

func makeSearchUsernameErrorResponse(ctx context.Context, cause string) ([]byte, error) {
	type errorResponse struct {
		Success bool   `json:"success"`
		Cause   string `json:"cause"`
	}
	resp := errorResponse{
		Success: false,
		Cause:   cause,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return data, nil
}
