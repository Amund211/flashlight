package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
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
	Success                bool               `json:"success"`
	UUID                   string             `json:"uuid,omitempty"`
	Year                   int                `json:"year,omitempty"`
	TotalSessions          int                `json:"totalSessions"`
	NonConsecutiveSessions int                `json:"nonConsecutiveSessions"`
	YearStats              *yearBoundaryStats `json:"yearStats,omitempty"`
	SessionStats           *sessionStats      `json:"sessionStats,omitempty"`
	Cause                  string             `json:"cause,omitempty"`
}

// sessionStats contains statistics computed from consecutive sessions
// This field is only present when there is at least one consecutive session
type sessionStats struct {
	SessionLengths        sessionLengthStats        `json:"sessionLengths"`
	SessionsPerMonth      map[int]int               `json:"sessionsPerMonth"`
	BestSessions          bestSessionsStats         `json:"bestSessions"`
	Averages              averageStats              `json:"averages"`
	Winstreaks            winstreakStats            `json:"winstreaks"`
	FinalKillStreaks      finalKillStreakStats      `json:"finalKillStreaks"`
	SessionCoverage       coverageStats             `json:"sessionCoverage"`
	FavoritePlayIntervals []playIntervalStats       `json:"favoritePlayIntervals"`
	FlawlessSessions      flawlessSessionStats      `json:"flawlessSessions"`
	PlaytimeDistribution  playtimeDistributionStats `json:"playtimeDistribution"`
}

type sessionLengthStats struct {
	Total    float64 `json:"totalHours"`
	Longest  float64 `json:"longestHours"`
	Shortest float64 `json:"shortestHours"`
	Average  float64 `json:"averageHours"`
}

type bestSessionsStats struct {
	HighestFKDR       *sessionSummary `json:"highestFKDR,omitempty"`
	MostKills         *sessionSummary `json:"mostKills,omitempty"`
	MostFinalKills    *sessionSummary `json:"mostFinalKills,omitempty"`
	MostWins          *sessionSummary `json:"mostWins,omitempty"`
	LongestSession    *sessionSummary `json:"longestSession,omitempty"`
	MostWinsPerHour   *sessionSummary `json:"mostWinsPerHour,omitempty"`
	MostFinalsPerHour *sessionSummary `json:"mostFinalsPerHour,omitempty"`
}

type sessionSummary struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Duration float64   `json:"durationHours"`
	Stats    gameStats `json:"stats"`
	Value    float64   `json:"value"`
}

type gameStats struct {
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

type averageStats struct {
	SessionLength float64 `json:"sessionLengthHours"`
	GamesPlayed   float64 `json:"gamesPlayed"`
	Wins          float64 `json:"wins"`
	FinalKills    float64 `json:"finalKills"`
}

type yearBoundaryStats struct {
	Start *domain.PlayerPIT `json:"start,omitempty"`
	End   *domain.PlayerPIT `json:"end,omitempty"`
}

type winstreakStats struct {
	Overall *gamemodeWinstreak `json:"overall,omitempty"`
	Solo    *gamemodeWinstreak `json:"solo,omitempty"`
	Doubles *gamemodeWinstreak `json:"doubles,omitempty"`
	Threes  *gamemodeWinstreak `json:"threes,omitempty"`
	Fours   *gamemodeWinstreak `json:"fours,omitempty"`
}

type gamemodeWinstreak struct {
	Highest int       `json:"highest"`
	When    time.Time `json:"when"`
}

type finalKillStreakStats struct {
	Overall *gamemodeFinalKillStreak `json:"overall,omitempty"`
	Solo    *gamemodeFinalKillStreak `json:"solo,omitempty"`
	Doubles *gamemodeFinalKillStreak `json:"doubles,omitempty"`
	Threes  *gamemodeFinalKillStreak `json:"threes,omitempty"`
	Fours   *gamemodeFinalKillStreak `json:"fours,omitempty"`
}

type gamemodeFinalKillStreak struct {
	Highest int       `json:"highest"`
	When    time.Time `json:"when"`
}

type coverageStats struct {
	GamesPlayedPercentage float64 `json:"gamesPlayedPercentage"`
	AdjustedTotalHours    float64 `json:"adjustedTotalHours"`
}

type playIntervalStats struct {
	HourStart  int     `json:"hourStart"`
	HourEnd    int     `json:"hourEnd"`
	Percentage float64 `json:"percentage"`
}

type flawlessSessionStats struct {
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type playtimeDistributionStats struct {
	// HourlyDistribution: Array of 24 elements (index 0-23) for hours in the specified timezone
	// Each element contains the total hours played during that hour
	HourlyDistribution [24]float64 `json:"hourlyDistribution"`

	// DayHourDistribution: Map from weekday name (e.g., "Monday", "Tuesday") to hourly distribution
	// Each value is an array of 24 elements (index 0-23) for hours on that day in the specified timezone
	// Keys are from time.Weekday.String(): "Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"
	DayHourDistribution map[string][24]float64 `json:"dayHourDistribution"`
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
			w.Write([]byte(`{"success":false,"cause":"invalid uuid"}`))
			return
		}

		year, err := strconv.Atoi(rawYear)
		if err != nil || year < 2000 || year > 2100 {
			statusCode := http.StatusBadRequest
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"invalid year"}`))
			return
		}

		// Parse and validate timezone parameter (optional, defaults to UTC)
		timezoneStr := r.URL.Query().Get("timezone")
		if timezoneStr == "" {
			timezoneStr = "UTC"
		}
		location, err := time.LoadLocation(timezoneStr)
		if err != nil {
			statusCode := http.StatusBadRequest
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"invalid timezone"}`))
			return
		}
		ctx = logging.AddMetaToContext(ctx,
			slog.String("timezone", timezoneStr),
		)

		ctx = reporting.AddExtrasToContext(ctx, map[string]string{
			"uuid": uuid,
		})
		ctx = logging.AddMetaToContext(ctx,
			slog.String("normalizedUUID", uuid),
			slog.Int("parsedYear", year),
		)

		yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		yearEnd := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)

		// NOTE: 24-hour padding to ensure we can complete sessions at year boundaries
		playerPITsStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Add(-24 * time.Hour)
		playerPITsEnd := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond).Add(24 * time.Hour)
		playerPITs, err := getPlayerPITs(ctx, uuid, playerPITsStart, playerPITsEnd)
		if err != nil {
			// NOTE: GetPlayerPITs implementations handle their own error reporting
			statusCode := http.StatusInternalServerError
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"success":false,"cause":"failed to get player data"}`))
			return
		}

		// Compute sessions from player PITs
		sessions := app.ComputeSessions(ctx, playerPITs, yearStart, yearEnd)

		// Compute wrapped statistics
		wrappedData := computeWrappedStats(ctx, playerPITs, sessions, year, location)
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
			w.Write([]byte(`{"success":false,"cause":"failed to marshal response"}`))
			return
		}

		logging.FromContext(ctx).InfoContext(ctx, "Returning wrapped data", "sessions", len(sessions))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(marshalled)
	}

	return middleware(handler)
}

func calculateSessionStats(start, end domain.GamemodeStatsPIT) gameStats {
	return gameStats{
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

func computeWrappedStats(ctx context.Context, playerPITs []domain.PlayerPIT, sessions []domain.Session, year int, location *time.Location) wrappedResponse {
	// Filter to consecutive sessions only
	consecutiveSessions := []domain.Session{}
	nonConsecutiveCount := 0

	for _, session := range sessions {
		if session.Consecutive {
			consecutiveSessions = append(consecutiveSessions, session)
		} else {
			nonConsecutiveCount++
		}
	}

	response := wrappedResponse{
		Year:                   year,
		TotalSessions:          len(consecutiveSessions),
		NonConsecutiveSessions: nonConsecutiveCount,
		YearStats:              computeYearBoundaryStats(ctx, playerPITs, year),
	}

	if len(consecutiveSessions) == 0 {
		return response
	}

	// Compute all session-dependent statistics
	response.SessionStats = &sessionStats{
		SessionLengths:        computeSessionLengths(ctx, consecutiveSessions),
		SessionsPerMonth:      computeSessionsPerMonth(ctx, consecutiveSessions),
		BestSessions:          computeBestSessions(ctx, consecutiveSessions),
		Averages:              computeAverages(ctx, consecutiveSessions),
		Winstreaks:            computeWinstreaks(ctx, playerPITs),
		FinalKillStreaks:      computeFinalKillStreaks(ctx, playerPITs),
		SessionCoverage:       computeCoverage(ctx, playerPITs, consecutiveSessions, year),
		FavoritePlayIntervals: computeFavoritePlayIntervals(ctx, consecutiveSessions),
		FlawlessSessions:      computeFlawlessSessions(ctx, consecutiveSessions),
		PlaytimeDistribution:  computePlaytimeDistribution(ctx, consecutiveSessions, location),
	}

	return response
}

// computeSessionLengths calculates session length statistics
// Assumes at least one session exists
func computeSessionLengths(ctx context.Context, sessions []domain.Session) sessionLengthStats {
	var total, longest, shortest float64
	shortest = -1

	for _, session := range sessions {
		duration := session.End.QueriedAt.Sub(session.Start.QueriedAt).Hours()
		total += duration
		if duration > longest {
			longest = duration
		}
		if shortest < 0 || duration < shortest {
			shortest = duration
		}
	}

	return sessionLengthStats{
		Total:    total,
		Longest:  longest,
		Shortest: shortest,
		Average:  total / float64(len(sessions)),
	}
}

// computeSessionsPerMonth returns count of sessions per month (1-12)
func computeSessionsPerMonth(ctx context.Context, sessions []domain.Session) map[int]int {
	counts := make(map[int]int)

	for _, session := range sessions {
		month := int(session.Start.QueriedAt.Month())
		counts[month]++
	}

	return counts
}

// computeBestSessions finds sessions with best performance in various categories
// Assumes at least one session exists
func computeBestSessions(ctx context.Context, sessions []domain.Session) bestSessionsStats {
	var bestFKDR, bestKills, bestFinals, bestWins, bestLongest *domain.Session
	var bestWinsPerHour, bestFinalsPerHour *domain.Session
	var maxFKDR, maxWinsPerHour, maxFinalsPerHour float64
	var maxKills, maxFinals, maxWins int
	var maxDuration time.Duration

	minSessionDuration := 15 * time.Minute
	minWins := 2
	minFinals := 10

	for i := range sessions {
		session := &sessions[i]
		stats := calculateSessionStats(session.Start.Overall, session.End.Overall)
		duration := session.End.QueriedAt.Sub(session.Start.QueriedAt)
		durationHours := duration.Hours()

		// FKDR
		// If no final deaths, use final kills as FKDR
		fkdr := float64(stats.FinalKills)
		if stats.FinalDeaths > 0 {
			fkdr = float64(stats.FinalKills) / float64(stats.FinalDeaths)
		}
		if bestFKDR == nil || fkdr > maxFKDR {
			bestFKDR = session
			maxFKDR = fkdr
		}

		// Most kills
		if bestKills == nil || stats.Kills > maxKills {
			bestKills = session
			maxKills = stats.Kills
		}

		// Most final kills
		if bestFinals == nil || stats.FinalKills > maxFinals {
			bestFinals = session
			maxFinals = stats.FinalKills
		}

		// Most wins
		if bestWins == nil || stats.Wins > maxWins {
			bestWins = session
			maxWins = stats.Wins
		}

		// Longest session
		if bestLongest == nil || duration > maxDuration {
			bestLongest = session
			maxDuration = duration
		}

		// Wins per hour (only if session is long enough and has enough wins)
		if duration >= minSessionDuration && stats.Wins >= minWins && durationHours > 0 {
			winsPerHour := float64(stats.Wins) / durationHours
			if bestWinsPerHour == nil || winsPerHour > maxWinsPerHour {
				bestWinsPerHour = session
				maxWinsPerHour = winsPerHour
			}
		}

		// Finals per hour (only if session is long enough and has enough finals)
		if duration >= minSessionDuration && stats.FinalKills >= minFinals && durationHours > 0 {
			finalsPerHour := float64(stats.FinalKills) / durationHours
			if bestFinalsPerHour == nil || finalsPerHour > maxFinalsPerHour {
				bestFinalsPerHour = session
				maxFinalsPerHour = finalsPerHour
			}
		}
	}

	result := bestSessionsStats{}

	if bestFKDR != nil {
		stats := calculateSessionStats(bestFKDR.Start.Overall, bestFKDR.End.Overall)
		duration := bestFKDR.End.QueriedAt.Sub(bestFKDR.Start.QueriedAt)
		result.HighestFKDR = &sessionSummary{
			Start:    bestFKDR.Start.QueriedAt,
			End:      bestFKDR.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    stats,
			Value:    maxFKDR,
		}
	}

	if bestKills != nil {
		stats := calculateSessionStats(bestKills.Start.Overall, bestKills.End.Overall)
		duration := bestKills.End.QueriedAt.Sub(bestKills.Start.QueriedAt)
		result.MostKills = &sessionSummary{
			Start:    bestKills.Start.QueriedAt,
			End:      bestKills.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    stats,
			Value:    float64(maxKills),
		}
	}

	if bestFinals != nil {
		stats := calculateSessionStats(bestFinals.Start.Overall, bestFinals.End.Overall)
		duration := bestFinals.End.QueriedAt.Sub(bestFinals.Start.QueriedAt)
		result.MostFinalKills = &sessionSummary{
			Start:    bestFinals.Start.QueriedAt,
			End:      bestFinals.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    stats,
			Value:    float64(maxFinals),
		}
	}

	if bestWins != nil {
		stats := calculateSessionStats(bestWins.Start.Overall, bestWins.End.Overall)
		duration := bestWins.End.QueriedAt.Sub(bestWins.Start.QueriedAt)
		result.MostWins = &sessionSummary{
			Start:    bestWins.Start.QueriedAt,
			End:      bestWins.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    stats,
			Value:    float64(maxWins),
		}
	}

	if bestLongest != nil {
		stats := calculateSessionStats(bestLongest.Start.Overall, bestLongest.End.Overall)
		result.LongestSession = &sessionSummary{
			Start:    bestLongest.Start.QueriedAt,
			End:      bestLongest.End.QueriedAt,
			Duration: maxDuration.Hours(),
			Stats:    stats,
			Value:    maxDuration.Hours(),
		}
	}

	if bestWinsPerHour != nil {
		stats := calculateSessionStats(bestWinsPerHour.Start.Overall, bestWinsPerHour.End.Overall)
		duration := bestWinsPerHour.End.QueriedAt.Sub(bestWinsPerHour.Start.QueriedAt)
		result.MostWinsPerHour = &sessionSummary{
			Start:    bestWinsPerHour.Start.QueriedAt,
			End:      bestWinsPerHour.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    stats,
			Value:    maxWinsPerHour,
		}
	}

	if bestFinalsPerHour != nil {
		stats := calculateSessionStats(bestFinalsPerHour.Start.Overall, bestFinalsPerHour.End.Overall)
		duration := bestFinalsPerHour.End.QueriedAt.Sub(bestFinalsPerHour.Start.QueriedAt)
		result.MostFinalsPerHour = &sessionSummary{
			Start:    bestFinalsPerHour.Start.QueriedAt,
			End:      bestFinalsPerHour.End.QueriedAt,
			Duration: duration.Hours(),
			Stats:    stats,
			Value:    maxFinalsPerHour,
		}
	}

	return result
}

// computeAverages calculates average statistics across all sessions
// Assumes at least one session exists
func computeAverages(ctx context.Context, sessions []domain.Session) averageStats {
	var totalDuration float64
	var totalGames, totalWins, totalFinalKills int

	for _, session := range sessions {
		duration := session.End.QueriedAt.Sub(session.Start.QueriedAt).Hours()
		totalDuration += duration

		stats := calculateSessionStats(session.Start.Overall, session.End.Overall)
		totalGames += stats.GamesPlayed
		totalWins += stats.Wins
		totalFinalKills += stats.FinalKills
	}

	numSessions := float64(len(sessions))
	return averageStats{
		SessionLength: totalDuration / numSessions,
		GamesPlayed:   float64(totalGames) / numSessions,
		Wins:          float64(totalWins) / numSessions,
		FinalKills:    float64(totalFinalKills) / numSessions,
	}
}

// computeYearBoundaryStats finds the first and last player stats in the year
func computeYearBoundaryStats(ctx context.Context, playerPITs []domain.PlayerPIT, year int) *yearBoundaryStats {
	if len(playerPITs) == 0 {
		return nil
	}

	yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)

	var firstPIT, lastPIT *domain.PlayerPIT

	for i := range playerPITs {
		pit := &playerPITs[i]
		if pit.QueriedAt.Before(yearStart) || pit.QueriedAt.After(yearEnd) {
			continue
		}

		if firstPIT == nil || pit.QueriedAt.Before(firstPIT.QueriedAt) {
			firstPIT = pit
		}
		if lastPIT == nil || pit.QueriedAt.After(lastPIT.QueriedAt) {
			lastPIT = pit
		}
	}

	if firstPIT == nil && lastPIT == nil {
		return nil
	}

	return &yearBoundaryStats{
		Start: firstPIT,
		End:   lastPIT,
	}
}

// computeWinstreaks calculates the highest winstreak for each gamemode during the year
// computeWinstreaks calculates the highest winstreak for each gamemode during the year
// Assumes at least one playerPIT exists
func computeWinstreaks(ctx context.Context, playerPITs []domain.PlayerPIT) winstreakStats {
	result := winstreakStats{}

	// Helper to compute winstreak for a gamemode
	computeGamemodeWinstreak := func(getStats func(*domain.PlayerPIT) domain.GamemodeStatsPIT) *gamemodeWinstreak {
		firstPlayer := &playerPITs[0]
		firstStats := getStats(firstPlayer)

		maxEndedStreak := 0
		maxStreakLastHeld := firstPlayer.QueriedAt

		currentStreak := 0
		prevWins := firstStats.Wins
		prevLosses := firstStats.Losses
		prevQueriedAt := firstPlayer.QueriedAt

		for i := range playerPITs {
			pit := &playerPITs[i]
			stats := getStats(pit)

			wins := stats.Wins
			losses := stats.Losses

			winsGained := wins - prevWins
			lossesGained := losses - prevLosses

			if lossesGained > 0 {
				// Streak broken

				// Don't count ongoing streaks
				if currentStreak > maxEndedStreak {
					maxEndedStreak = currentStreak
					maxStreakLastHeld = prevQueriedAt
				}

				currentStreak = 0
			} else if winsGained > 0 {
				// Streak continues/starts
				currentStreak += winsGained
			}

			prevWins = wins
			prevLosses = losses
			prevQueriedAt = pit.QueriedAt
		}

		return &gamemodeWinstreak{
			Highest: maxEndedStreak,
			When:    maxStreakLastHeld,
		}
	}

	result.Overall = computeGamemodeWinstreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Overall })
	result.Solo = computeGamemodeWinstreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Solo })
	result.Doubles = computeGamemodeWinstreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Doubles })
	result.Threes = computeGamemodeWinstreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Threes })
	result.Fours = computeGamemodeWinstreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Fours })

	return result
}

// computeFinalKillStreaks calculates the highest final kill streak for each gamemode during the year
// Assumes at least one playerPIT exists
func computeFinalKillStreaks(ctx context.Context, playerPITs []domain.PlayerPIT) finalKillStreakStats {
	result := finalKillStreakStats{}

	// Helper to compute final kill streak for a gamemode
	computeGamemodeFKStreak := func(getStats func(*domain.PlayerPIT) domain.GamemodeStatsPIT) *gamemodeFinalKillStreak {
		firstPlayer := &playerPITs[0]
		firstStats := getStats(firstPlayer)

		maxEndedStreak := 0
		maxStreakEndedAt := firstPlayer.QueriedAt

		currentStreak := 0
		prevFKills := firstStats.FinalKills
		prevFDeaths := firstStats.FinalDeaths
		prevQueriedAt := firstPlayer.QueriedAt

		for i := range playerPITs {
			pit := &playerPITs[i]
			stats := getStats(pit)

			fkills := stats.FinalKills
			fdeaths := stats.FinalDeaths

			fkillsGained := fkills - prevFKills
			fdeathsGained := fdeaths - prevFDeaths

			if fdeathsGained > 0 {
				// Streak broken

				// Don't count ongoing streaks
				if currentStreak > maxEndedStreak {
					maxEndedStreak = currentStreak
					maxStreakEndedAt = prevQueriedAt
				}

				currentStreak = 0
			} else if fkillsGained > 0 {
				// Streak continues/starts
				currentStreak += fkillsGained
			}

			prevFKills = fkills
			prevFDeaths = fdeaths
			prevQueriedAt = pit.QueriedAt
		}

		return &gamemodeFinalKillStreak{
			Highest: maxEndedStreak,
			When:    maxStreakEndedAt,
		}
	}

	result.Overall = computeGamemodeFKStreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Overall })
	result.Solo = computeGamemodeFKStreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Solo })
	result.Doubles = computeGamemodeFKStreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Doubles })
	result.Threes = computeGamemodeFKStreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Threes })
	result.Fours = computeGamemodeFKStreak(func(p *domain.PlayerPIT) domain.GamemodeStatsPIT { return p.Fours })

	return result
}

// computeCoverage calculates what percentage of stats were covered by sessions
// Assumes at least one playerPIT and session exists
func computeCoverage(ctx context.Context, playerPITs []domain.PlayerPIT, sessions []domain.Session, year int) coverageStats {
	yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)

	// Find first and last PIT in year
	var firstPIT, lastPIT *domain.PlayerPIT
	for i := range playerPITs {
		pit := &playerPITs[i]
		if pit.QueriedAt.Before(yearStart) || pit.QueriedAt.After(yearEnd) {
			continue
		}
		if firstPIT == nil || pit.QueriedAt.Before(firstPIT.QueriedAt) {
			firstPIT = pit
		}
		if lastPIT == nil || pit.QueriedAt.After(lastPIT.QueriedAt) {
			lastPIT = pit
		}
	}

	// If no PITs in year range, return zero coverage
	if firstPIT == nil || lastPIT == nil {
		return coverageStats{
			GamesPlayedPercentage: 0,
			AdjustedTotalHours:    0,
		}
	}

	// Total games played in year
	totalGames := lastPIT.Overall.GamesPlayed - firstPIT.Overall.GamesPlayed
	if totalGames <= 0 {
		return coverageStats{
			GamesPlayedPercentage: 0,
			AdjustedTotalHours:    0,
		}
	}

	// Games covered by sessions
	sessionGames := 0
	var sessionDuration float64
	for _, session := range sessions {
		stats := calculateSessionStats(session.Start.Overall, session.End.Overall)
		sessionGames += stats.GamesPlayed
		sessionDuration += session.End.QueriedAt.Sub(session.Start.QueriedAt).Hours()
	}

	coverage := float64(sessionGames) / float64(totalGames) * 100

	// Adjust total session time based on coverage
	adjustedHours := sessionDuration
	if coverage > 0 && coverage < 100 {
		adjustedHours = sessionDuration / (coverage / 100)
	} else if coverage > 100 {
		// Coverage exceeded 100%, likely due to data inconsistency
		// Keep sessionDuration as-is
		adjustedHours = sessionDuration
	}

	return coverageStats{
		GamesPlayedPercentage: coverage,
		AdjustedTotalHours:    adjustedHours,
	}
}

// computeFavoritePlayIntervals calculates which time intervals have most playtime
// Assumes at least one session exists
func computeFavoritePlayIntervals(ctx context.Context, sessions []domain.Session) []playIntervalStats {
	// Track playtime in each hour bucket (0-23)
	hourBuckets := make([]float64, 24)
	var totalTime float64

	for _, session := range sessions {
		duration := session.End.QueriedAt.Sub(session.Start.QueriedAt)
		totalTime += duration.Hours()

		// Distribute time across hours
		start := session.Start.QueriedAt
		end := session.End.QueriedAt

		for t := start; t.Before(end); {
			hour := t.Hour()
			// Find the next hour boundary
			nextHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			segmentEnd := end
			if nextHour.Before(end) {
				segmentEnd = nextHour
			}
			hourBuckets[hour] += segmentEnd.Sub(t).Hours()
			t = segmentEnd
		}
	}

	if totalTime == 0 {
		return []playIntervalStats{}
	}

	// Find top 3 intervals
	type interval struct {
		start      int
		end        int
		percentage float64
	}

	intervals := []interval{}

	// Try 4-hour windows
	for start := 0; start < 24; start++ {
		var windowTime float64
		for i := 0; i < 4; i++ {
			hour := (start + i) % 24
			windowTime += hourBuckets[hour]
		}
		percentage := windowTime / totalTime * 100
		if percentage > 5 { // Only include if > 5%
			intervals = append(intervals, interval{
				start:      start,
				end:        (start + 4) % 24,
				percentage: percentage,
			})
		}
	}

	// Sort by percentage (descending)
	slices.SortFunc(intervals, func(a, b interval) int {
		if a.percentage > b.percentage {
			return -1
		} else if a.percentage < b.percentage {
			return 1
		}
		return 0
	})

	// Take top 3
	result := []playIntervalStats{}
	for i := 0; i < len(intervals) && i < 3; i++ {
		result = append(result, playIntervalStats{
			HourStart:  intervals[i].start,
			HourEnd:    intervals[i].end,
			Percentage: intervals[i].percentage,
		})
	}

	return result
}

// computeFlawlessSessions counts sessions with no losses and no final deaths
// Assumes at least one session exists
func computeFlawlessSessions(ctx context.Context, sessions []domain.Session) flawlessSessionStats {
	flawlessCount := 0
	for _, session := range sessions {
		stats := calculateSessionStats(session.Start.Overall, session.End.Overall)
		if stats.Losses == 0 && stats.FinalDeaths == 0 && stats.Wins > 0 {
			flawlessCount++
		}
	}

	percentage := float64(flawlessCount) / float64(len(sessions)) * 100

	return flawlessSessionStats{
		Count:      flawlessCount,
		Percentage: percentage,
	}
}

// computePlaytimeDistribution calculates the distribution of playtime across UTC hours and day-hours
// Assumes at least one session exists
func computePlaytimeDistribution(ctx context.Context, sessions []domain.Session, location *time.Location) playtimeDistributionStats {
	var hourlyDistribution [24]float64
	dayHourDistribution := make(map[string][24]float64)

	for _, session := range sessions {
		// Convert UTC times to the specified timezone
		start := session.Start.QueriedAt.In(location)
		end := session.End.QueriedAt.In(location)

		// Distribute session time across hour buckets
		currentTime := start
		for currentTime.Before(end) {
			hour := currentTime.Hour()
			weekdayName := currentTime.Weekday().String()

			// Calculate the end of the current hour
			nextHour := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), currentTime.Hour(), 0, 0, 0, currentTime.Location()).Add(time.Hour)

			// Determine how much time to add for this hour bucket
			var hoursToAdd float64
			if end.Before(nextHour) {
				// Session ends before the next hour boundary
				hoursToAdd = end.Sub(currentTime).Hours()
			} else {
				// Session continues into next hour
				hoursToAdd = nextHour.Sub(currentTime).Hours()
			}

			hourlyDistribution[hour] += hoursToAdd

			// Get or create the hourly distribution for this weekday
			dayHours := dayHourDistribution[weekdayName]
			dayHours[hour] += hoursToAdd
			dayHourDistribution[weekdayName] = dayHours

			// Move to the next hour boundary
			currentTime = nextHour
		}
	}

	return playtimeDistributionStats{
		HourlyDistribution:  hourlyDistribution,
		DayHourDistribution: dayHourDistribution,
	}
}
