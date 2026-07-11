package ports

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

// MakeGetLatestSessionHandler serves POST /v1/session-at/latest. It returns the
// same response body as the session-at handler, but takes only a uuid and
// resolves the time itself (the player's latest session). See
// app.BuildGetLatestSession.
func MakeGetLatestSessionHandler(
	getLatestSession app.GetLatestSession,
	registerUserVisit app.RegisterUserVisit,
	allowedOrigins *DomainSuffixes,
	rootLogger *slog.Logger,
	sentryMiddleware func(http.HandlerFunc) http.HandlerFunc,
	blocklistConfig BlocklistConfig,
) http.HandlerFunc {
	ipLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(4),
		ratelimiting.BurstSize(80),
	)
	ipRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		ipLimiter,
		IPHashKeyFunc,
	)
	userIDLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(1),
		ratelimiting.BurstSize(20),
	)
	userIDRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		userIDLimiter,
		UserIDKeyFunc,
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
		NewRequestLoggerMiddleware(rootLogger),
		sentryMiddleware,
		BuildBlocklistMiddleware(blocklistConfig),
		buildMetricsMiddleware("session-at-latest"),
		NewReportingMetaMiddleware("session-at-latest"),
		BuildCORSMiddleware(allowedOrigins),
		NewRateLimitMiddleware(ipRateLimiter, makeOnLimitExceeded(ipRateLimiter)),
		NewRateLimitMiddleware(userIDRateLimiter, makeOnLimitExceeded(userIDRateLimiter)),
		BuildRegisterUserVisitMiddleware(registerUserVisit),
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		defer r.Body.Close()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<10))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			reporting.Report(ctx, fmt.Errorf("failed to read request body: %w", err))
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		request := struct {
			UUID string `json:"uuid"`
		}{}
		err = json.Unmarshal(body, &request)
		if err != nil {
			logging.FromContext(ctx).WarnContext(ctx, "Failed to parse request body", "error", err)
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		uuid, err := strutils.NormalizeUUID(request.UUID)
		if err != nil {
			logging.FromContext(ctx).WarnContext(ctx, "Failed to normalize uuid", "error", err, "rawUUID", request.UUID)
			http.Error(w, "invalid uuid", http.StatusBadRequest)
			return
		}

		logging.FromContext(ctx).InfoContext(ctx, "Handling latest-session request",
			slog.String("uuid", uuid),
		)

		ctx = reporting.AddExtrasToContext(ctx, map[string]string{
			"uuid": uuid,
		})
		ctx = logging.AddMetaToContext(ctx,
			slog.String("uuid", uuid),
		)

		result, err := getLatestSession(ctx, uuid)
		if err != nil {
			// NOTE: GetLatestSession implementations handle their own error reporting
			http.Error(w, "Failed to get session", http.StatusInternalServerError)
			return
		}

		marshalled, err := marshalRainbowSessionAtResponse(ctx, result)
		if err != nil {
			// NOTE: marshalRainbowSessionAtResponse handles its own error reporting
			http.Error(w, "Failed to serialise response", http.StatusInternalServerError)
			return
		}

		logging.FromContext(ctx).InfoContext(ctx, "Returning latest session",
			"hasSession", result.Session != nil,
			"gamesLength", len(result.Games),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(marshalled)
	}

	return middleware(handler)
}
