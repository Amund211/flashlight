package ports

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

type wrappedResponse struct {
	Success        bool                   `json:"success"`
	UUID           string                 `json:"uuid,omitempty"`
	Year           int                    `json:"year,omitempty"`
	TotalSessions  int                    `json:"totalSessions"`
	LongestSession *wrappedSessionSummary `json:"longestSession,omitempty"`
	HighestFKDR    *wrappedSessionSummary `json:"highestFKDR,omitempty"`
	TotalStats     *wrappedStats          `json:"totalStats,omitempty"`
	Cause          string                 `json:"cause,omitempty"`
}

type wrappedSessionSummary struct {
	Start    time.Time    `json:"start"`
	End      time.Time    `json:"end"`
	Duration float64      `json:"durationHours"`
	Stats    wrappedStats `json:"stats"`
	FKDR     *float64     `json:"fkdr,omitempty"`
}

type wrappedStats struct {
	GamesPlayed int `json:"gamesPlayed"`
	Wins        int `json:"wins"`
	Losses      int `json:"losses"`
	BedsBroken  int `json:"bedsBroken"`
	BedsLost    int `json:"bedsLost"`
	FinalKills  int `json:"finalKills"`
	FinalDeaths int `json:"finalDeaths"`
	Kills       int `json:"kills"`
	Deaths      int `json:"deaths"`
}

func MakeGetWrappedHandler(
	getPlayerPITs app.GetPlayerPITs,
	allowedOrigins *DomainSuffixes,
	rootLogger *slog.Logger,
	sentryMiddleware func(http.HandlerFunc) http.HandlerFunc,
) http.HandlerFunc {
	ipLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(4),
		ratelimiting.BurstSize(240),
	)
	ipRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		ipLimiter,
		ratelimiting.IPKeyFunc,
	)
	userIDLimiter, _ := ratelimiting.NewTokenBucketRateLimiter(
		ratelimiting.RefillPerSecond(1),
		ratelimiting.BurstSize(60),
	)
	userIDRateLimiter := ratelimiting.NewRequestBasedRateLimiter(
		// NOTE: Rate limiting based on user controlled value
		userIDLimiter,
		ratelimiting.UserIDKeyFunc,
	)

	makeOnLimitExceeded := func(rateLimiter ratelimiting.RequestRateLimiter) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"cause":"Rate limit exceeded"}`))
		}
	}

	middleware := ComposeMiddlewares(
		buildMetricsMiddleware("wrapped"),
		logging.NewRequestLoggerMiddleware(rootLogger),
		sentryMiddleware,
		reporting.NewAddMetaMiddleware("wrapped"),
		BuildCORSMiddleware(allowedOrigins),
		NewRateLimitMiddleware(ipRateLimiter, makeOnLimitExceeded(ipRateLimiter)),
		NewRateLimitMiddleware(userIDRateLimiter, makeOnLimitExceeded(userIDRateLimiter)),
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		rawUUID := r.PathValue("uuid")
		rawYear := r.PathValue("year")
		userID := r.Header.Get("X-User-Id")
		ctx = reporting.SetUserIDInContext(ctx, userID)
		if userID == "" {
			userID = "<missing>"
		}
		ctx = logging.AddMetaToContext(ctx,
			slog.String("userId", userID),
			slog.String("uuid", rawUUID),
			slog.String("year", rawYear),
		)

		ctx = reporting.AddExtrasToContext(ctx,
			map[string]string{
				"rawUUID": rawUUID,
				"year":    rawYear,
			},
		)

		uuid, err := strutils.NormalizeUUID(rawUUID)
		if err != nil {
			statusCode := http.StatusBadRequest
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"Invalid UUID"}`))
			return
		}

		year, err := strconv.Atoi(rawYear)
		if err != nil || year < 2000 || year > 2100 {
			statusCode := http.StatusBadRequest
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"Invalid year"}`))
			return
		}

		ctx = reporting.AddExtrasToContext(ctx, map[string]string{
			"uuid": uuid,
		})
		ctx = logging.AddMetaToContext(ctx,
			slog.String("normalizedUUID", uuid),
			slog.Int("parsedYear", year),
		)

		// Calculate start and end times for the year
		// Add 24 hour padding as per existing pattern
		start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Add(-24 * time.Hour)
		end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond).Add(24 * time.Hour)

		// Get player PITs for the year
		playerPITs, err := getPlayerPITs(ctx, uuid, start, end)
		if err != nil {
			// NOTE: GetPlayerPITs implementations handle their own error reporting
			statusCode := http.StatusInternalServerError
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"Failed to get player data"}`))
			return
		}

		// Compute sessions from player PITs
		yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		yearEnd := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)
		sessions := app.ComputeSessions(ctx, playerPITs, yearStart, yearEnd)

		// Compute wrapped statistics
		wrappedData := computeWrappedStats(sessions, year)
		wrappedData.Success = true
		wrappedData.UUID = uuid

		marshalled, err := json.Marshal(wrappedData)
		if err != nil {
			reporting.Report(ctx, fmt.Errorf("failed to marshal wrapped response: %w", err), map[string]string{
				"sessionsCount": strconv.Itoa(len(sessions)),
			})
			statusCode := http.StatusInternalServerError
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"Failed to marshal response"}`))
			return
		}

		logging.FromContext(ctx).InfoContext(ctx, "Returning wrapped data", "sessions", len(sessions))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(marshalled)
	}

	return middleware(handler)
}

func calculateSessionStats(start, end domain.GamemodeStatsPIT) wrappedStats {
	return wrappedStats{
		GamesPlayed: end.GamesPlayed - start.GamesPlayed,
		Wins:        end.Wins - start.Wins,
		Losses:      end.Losses - start.Losses,
		BedsBroken:  end.BedsBroken - start.BedsBroken,
		BedsLost:    end.BedsLost - start.BedsLost,
		FinalKills:  end.FinalKills - start.FinalKills,
		FinalDeaths: end.FinalDeaths - start.FinalDeaths,
		Kills:       end.Kills - start.Kills,
		Deaths:      end.Deaths - start.Deaths,
	}
}

func computeWrappedStats(sessions []domain.Session, year int) wrappedResponse {
	response := wrappedResponse{
		Year:          year,
		TotalSessions: len(sessions),
	}

	if len(sessions) == 0 {
		return response
	}

	// Initialize totals
	totalStats := wrappedStats{}

	var longestSession *domain.Session
	var longestDuration time.Duration

	var highestFKDRSession *domain.Session
	var highestFKDR float64

	// Process each session
	for i := range sessions {
		session := &sessions[i]

		// Calculate stats delta for this session
		sessionStats := calculateSessionStats(session.Start.Overall, session.End.Overall)

		// Add to totals
		totalStats.GamesPlayed += sessionStats.GamesPlayed
		totalStats.Wins += sessionStats.Wins
		totalStats.Losses += sessionStats.Losses
		totalStats.BedsBroken += sessionStats.BedsBroken
		totalStats.BedsLost += sessionStats.BedsLost
		totalStats.FinalKills += sessionStats.FinalKills
		totalStats.FinalDeaths += sessionStats.FinalDeaths
		totalStats.Kills += sessionStats.Kills
		totalStats.Deaths += sessionStats.Deaths

		// Calculate session duration
		duration := session.End.QueriedAt.Sub(session.Start.QueriedAt)
		if longestSession == nil || duration > longestDuration {
			longestSession = session
			longestDuration = duration
		}

		// Calculate FKDR for this session
		if sessionStats.FinalDeaths > 0 {
			fkdr := float64(sessionStats.FinalKills) / float64(sessionStats.FinalDeaths)
			if highestFKDRSession == nil || fkdr > highestFKDR {
				highestFKDRSession = session
				highestFKDR = fkdr
			}
		}
	}

	response.TotalStats = &totalStats

	// Create longest session summary
	if longestSession != nil {
		sessionStats := calculateSessionStats(longestSession.Start.Overall, longestSession.End.Overall)

		response.LongestSession = &wrappedSessionSummary{
			Start:    longestSession.Start.QueriedAt,
			End:      longestSession.End.QueriedAt,
			Duration: longestDuration.Hours(),
			Stats:    sessionStats,
		}
	}

	// Create highest FKDR session summary
	if highestFKDRSession != nil {
		sessionStats := calculateSessionStats(highestFKDRSession.Start.Overall, highestFKDRSession.End.Overall)

		duration := highestFKDRSession.End.QueriedAt.Sub(highestFKDRSession.Start.QueriedAt)

		response.HighestFKDR = &wrappedSessionSummary{
			Start:    highestFKDRSession.Start.QueriedAt,
			End:      highestFKDRSession.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    sessionStats,
			FKDR:     &highestFKDR,
		}
	}

	return response
}
