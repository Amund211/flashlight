package ports

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
)

type rainbowStatsPIT struct {
	Winstreak   *int `json:"winstreak"`
	GamesPlayed int  `json:"gamesPlayed"`
	Wins        int  `json:"wins"`
	Losses      int  `json:"losses"`
	BedsBroken  int  `json:"bedsBroken"`
	BedsLost    int  `json:"bedsLost"`
	FinalKills  int  `json:"finalKills"`
	FinalDeaths int  `json:"finalDeaths"`
	Kills       int  `json:"kills"`
	Deaths      int  `json:"deaths"`
}

type rainbowPlayerDataPIT struct {
	UUID       string          `json:"uuid"`
	QueriedAt  time.Time       `json:"queriedAt"`
	Experience int64           `json:"experience"`
	Solo       rainbowStatsPIT `json:"solo"`
	Doubles    rainbowStatsPIT `json:"doubles"`
	Threes     rainbowStatsPIT `json:"threes"`
	Fours      rainbowStatsPIT `json:"fours"`
	Overall    rainbowStatsPIT `json:"overall"`
}

type rainbowSession struct {
	Start       rainbowPlayerDataPIT `json:"start"`
	End         rainbowPlayerDataPIT `json:"end"`
	Consecutive bool                 `json:"consecutive"`
}

func gamemodeStatsPITToRainbowStatsPIT(stats *domain.GamemodeStatsPIT) rainbowStatsPIT {
	return rainbowStatsPIT{
		Winstreak:   stats.Winstreak,
		GamesPlayed: stats.GamesPlayed,
		Wins:        stats.Wins,
		Losses:      stats.Losses,
		BedsBroken:  stats.BedsBroken,
		BedsLost:    stats.BedsLost,
		FinalKills:  stats.FinalKills,
		FinalDeaths: stats.FinalDeaths,
		Kills:       stats.Kills,
		Deaths:      stats.Deaths,
	}
}

func playerToRainbowPlayerDataPIT(player *domain.PlayerPIT) rainbowPlayerDataPIT {
	return rainbowPlayerDataPIT{
		UUID:       player.UUID,
		QueriedAt:  player.QueriedAt,
		Experience: player.Experience,
		Solo:       gamemodeStatsPITToRainbowStatsPIT(&player.Solo),
		Doubles:    gamemodeStatsPITToRainbowStatsPIT(&player.Doubles),
		Threes:     gamemodeStatsPITToRainbowStatsPIT(&player.Threes),
		Fours:      gamemodeStatsPITToRainbowStatsPIT(&player.Fours),
		Overall:    gamemodeStatsPITToRainbowStatsPIT(&player.Overall),
	}
}

func PlayerToRainbowPlayerDataPITData(player *domain.PlayerPIT) ([]byte, error) {
	if player == nil {
		return nil, fmt.Errorf("player is nil")
	}
	playerDataJSON, err := json.Marshal(playerToRainbowPlayerDataPIT(player))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal player data: %w", err)
	}
	return playerDataJSON, nil
}

func historyToRainbowHistory(history []domain.PlayerPIT) []rainbowPlayerDataPIT {
	rainbowHistory := make([]rainbowPlayerDataPIT, 0, len(history))

	for _, player := range history {
		rainbowHistory = append(rainbowHistory, playerToRainbowPlayerDataPIT(&player))
	}

	return rainbowHistory
}

func HistoryToRainbowHistoryData(history []domain.PlayerPIT) ([]byte, error) {
	historyDataJSON, err := json.Marshal(historyToRainbowHistory(history))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal history data: %w", err)
	}
	return historyDataJSON, nil
}

func sessionToRainbowSession(session *domain.Session) rainbowSession {
	return rainbowSession{
		Start:       playerToRainbowPlayerDataPIT(&session.Start),
		End:         playerToRainbowPlayerDataPIT(&session.End),
		Consecutive: session.Consecutive,
	}
}

func sessionsToRainbowSessions(sessions []domain.Session) []rainbowSession {
	rainbowSessions := make([]rainbowSession, 0, len(sessions))

	for _, session := range sessions {
		rainbowSessions = append(rainbowSessions, sessionToRainbowSession(&session))
	}

	return rainbowSessions
}

type rainbowSessionComputedStats struct {
	TotalSessions            int                   `json:"totalSessions"`
	TotalConsecutiveSessions int                   `json:"totalConsecutiveSessions"`
	TimeHistogram            [24]int               `json:"timeHistogram"`
	StatsAtYearStart         *rainbowPlayerDataPIT `json:"statsAtYearStart"`
	StatsAtYearEnd           *rainbowPlayerDataPIT `json:"statsAtYearEnd"`
}

type rainbowSessionsResponse struct {
	Sessions      []rainbowSession              `json:"sessions"`
	ComputedStats *rainbowSessionComputedStats `json:"computedStats,omitempty"`
}

func SessionsToRainbowSessionsData(sessions []domain.Session) ([]byte, error) {
	sessionsDataJSON, err := json.Marshal(sessionsToRainbowSessions(sessions))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sessions data: %w", err)
	}
	return sessionsDataJSON, nil
}

func SessionsToRainbowSessionsDataWithStats(
	sessions []domain.Session,
	stats []domain.PlayerPIT,
	year int,
	totalSessions int,
	totalConsecutiveSessions int,
	timeHistogram [24]int,
) ([]byte, error) {
	response := rainbowSessionsResponse{
		Sessions: sessionsToRainbowSessions(sessions),
	}

	// Only add computed stats if we have sessions
	if len(sessions) > 0 {
		statsAtYearStart := computeStatsAtYearStart(stats, year)
		statsAtYearEnd := computeStatsAtYearEnd(stats, year)

		var rainbowStatsStart *rainbowPlayerDataPIT
		if statsAtYearStart != nil {
			rainbowStats := playerToRainbowPlayerDataPIT(statsAtYearStart)
			rainbowStatsStart = &rainbowStats
		}

		var rainbowStatsEnd *rainbowPlayerDataPIT
		if statsAtYearEnd != nil {
			rainbowStats := playerToRainbowPlayerDataPIT(statsAtYearEnd)
			rainbowStatsEnd = &rainbowStats
		}

		response.ComputedStats = &rainbowSessionComputedStats{
			TotalSessions:            totalSessions,
			TotalConsecutiveSessions: totalConsecutiveSessions,
			TimeHistogram:            timeHistogram,
			StatsAtYearStart:         rainbowStatsStart,
			StatsAtYearEnd:           rainbowStatsEnd,
		}
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sessions response: %w", err)
	}
	return responseJSON, nil
}

func computeStatsAtYearStart(stats []domain.PlayerPIT, year int) *domain.PlayerPIT {
	var earliest *domain.PlayerPIT

	for i := range stats {
		stat := &stats[i]
		if stat.QueriedAt.Year() == year {
			if earliest == nil || stat.QueriedAt.Before(earliest.QueriedAt) {
				earliest = stat
			}
		}
	}

	return earliest
}

func computeStatsAtYearEnd(stats []domain.PlayerPIT, year int) *domain.PlayerPIT {
	var latest *domain.PlayerPIT

	for i := range stats {
		stat := &stats[i]
		if stat.QueriedAt.Year() == year {
			if latest == nil || stat.QueriedAt.After(latest.QueriedAt) {
				latest = stat
			}
		}
	}

	return latest
}
