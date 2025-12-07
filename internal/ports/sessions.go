package ports

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

// validateAndNormalizeTimezone validates the timezone string and returns the normalized version.
// If timezone is empty, returns "UTC". Returns error if the timezone is invalid.
func validateAndNormalizeTimezone(timezone string) (string, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	// Validate timezone by attempting to load it
	_, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone: %w", err)
	}
	return timezone, nil
}

func MakeGetSessionsHandler(
	getPlayerPITs app.GetPlayerPITs,
	allowedOrigins *DomainSuffixes,
	rootLogger *slog.Logger,
	sentryMiddleware func(http.HandlerFunc) http.HandlerFunc,
) http.HandlerFunc {
	ipLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(4),
		ratelimiting.BurstSize(80),
	)
	ipRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		ipLimiter,
		ratelimiting.IPKeyFunc,
	)
	userIDLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(1),
		ratelimiting.BurstSize(20),
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
		buildMetricsMiddleware("sessions"),
		logging.NewRequestLoggerMiddleware(rootLogger),
		sentryMiddleware,
		reporting.NewAddMetaMiddleware("sessions"),
		BuildCORSMiddleware(allowedOrigins),
		NewRateLimitMiddleware(ipRateLimiter, makeOnLimitExceeded(ipRateLimiter)),
		NewRateLimitMiddleware(userIDRateLimiter, makeOnLimitExceeded(userIDRateLimiter)),
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			userID = "<missing>"
		}
		ctx = reporting.SetUserIDInContext(ctx, userID)
		ctx = logging.AddMetaToContext(ctx, slog.String("userId", userID))

		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			reporting.Report(ctx, fmt.Errorf("failed to read request body: %w", err))
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		request := struct {
			UUID     string    `json:"uuid"`
			Start    time.Time `json:"start"`
			End      time.Time `json:"end"`
			Year     *int      `json:"year,omitempty"`
			Timezone string    `json:"timezone,omitempty"`
		}{}
		err = json.Unmarshal(body, &request)
		if err != nil {
			reporting.Report(ctx, fmt.Errorf("failed to parse request body: %w", err))
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		ctx = reporting.AddExtrasToContext(ctx, map[string]string{
			"start": request.Start.Format(time.RFC3339),
			"end":   request.End.Format(time.RFC3339),
		})

		uuid, err := strutils.NormalizeUUID(request.UUID)
		if err != nil {
			reporting.Report(ctx, fmt.Errorf("failed to normalize uuid: %w", err), map[string]string{
				"rawUUID": request.UUID,
			})
			http.Error(w, "invalid uuid", http.StatusBadRequest)
			return
		}

		ctx = reporting.AddExtrasToContext(ctx, map[string]string{
			"uuid": uuid,
		})
		ctx = logging.AddMetaToContext(ctx,
			slog.String("uuid", uuid),
			slog.String("start", request.Start.Format(time.RFC3339)),
			slog.String("end", request.End.Format(time.RFC3339)),
		)

		if request.Start.After(request.End) {
			reporting.Report(ctx, fmt.Errorf("start time is after end time"))
			http.Error(w, "Start time cannot be after end time", http.StatusBadRequest)
			return
		}

		// Validate interval length
		timespan := request.End.Sub(request.Start)
		// TODO: Revert to max 60 days (when no longer using this for "wrapped" page on website)
		if timespan >= 400*24*time.Hour {
			http.Error(w, "Time interval is too long", http.StatusBadRequest)
			return
		}

		// Validate timezone if provided (before making expensive data fetch)
		if request.Year != nil {
			_, err := validateAndNormalizeTimezone(request.Timezone)
			if err != nil {
				reporting.Report(ctx, err, map[string]string{
					"timezone": request.Timezone,
				})
				http.Error(w, "Invalid timezone", http.StatusBadRequest)
				return
			}
		}

		// Add some padding on both sides to try to complete sessions that cross the interval borders
		filterStart := request.Start.Add(-24 * time.Hour)
		filterEnd := request.End.Add(24 * time.Hour)

		stats, err := getPlayerPITs(ctx, uuid, filterStart, filterEnd)
		if err != nil {
			// NOTE: GetPlayerPITs implementations handle their own error reporting
			http.Error(w, "Failed to get player data", http.StatusInternalServerError)
			return
		}

		sessions := app.ComputeSessions(ctx, stats, request.Start, request.End)

		var marshalled []byte
		
		// If year is provided and we have sessions, compute additional stats
		if request.Year != nil && len(sessions) > 0 {
			// Normalize timezone (validated above, so this should not error)
			timezone, err := validateAndNormalizeTimezone(request.Timezone)
			if err != nil {
				reporting.Report(ctx, fmt.Errorf("unexpected error normalizing timezone: %w", err), map[string]string{
					"timezone": request.Timezone,
				})
				http.Error(w, "Failed to normalize timezone", http.StatusInternalServerError)
				return
			}
			
			totalSessions := len(sessions)
			totalConsecutiveSessions := app.ComputeTotalConsecutiveSessions(sessions)
			// We already validated the timezone above, so this should not error
			timeHistogram, err := app.ComputeTimeHistogram(sessions, timezone)
			if err != nil {
				reporting.Report(ctx, fmt.Errorf("unexpected error computing time histogram: %w", err), map[string]string{
					"timezone": timezone,
				})
				http.Error(w, "Failed to compute time histogram", http.StatusInternalServerError)
				return
			}
			
			marshalled, err = SessionsToRainbowSessionsDataWithStats(
				sessions,
				stats,
				*request.Year,
				totalSessions,
				totalConsecutiveSessions,
				timeHistogram,
			)
		} else {
			marshalled, err = SessionsToRainbowSessionsData(sessions)
		}
		
		if err != nil {
			reporting.Report(ctx, fmt.Errorf("failed to convert sessions to response: %w", err), map[string]string{
				"length": strconv.Itoa(len(sessions)),
			})
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}

		logging.FromContext(ctx).InfoContext(ctx, "Returning sessions data")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(marshalled)
	}

	return middleware(handler)
}
