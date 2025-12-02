package app

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/reporting"
)

// NOTE: All domain.PlayerPIT entries must be for the same player
func ComputeSessions(ctx context.Context, stats []domain.PlayerPIT, start, end time.Time) []domain.Session {
	if len(stats) <= 1 {
		// Need at least a start and end to create a session
		return []domain.Session{}
	}

	slices.SortStableFunc(stats, func(a, b domain.PlayerPIT) int {
		if a.QueriedAt.Before(b.QueriedAt) {
			return -1
		}
		if a.QueriedAt.After(b.QueriedAt) {
			return 1
		}
		return 0
	})

	sessions := []domain.Session{}

	getProgressStats := func(stat *domain.PlayerPIT) (int, int64) {
		return stat.Overall.GamesPlayed, stat.Experience
	}

	includeSession := func(sessionStart, lastEventfulEntry *domain.PlayerPIT) bool {
		if sessionStart == lastEventfulEntry {
			// Session starts and ends with the same entry -> not a session
			// NOTE: Using raw pointer comparison here, so we need to make sure we don't
			//       make any copies of the entries
			return false
		}

		if sessionStart.QueriedAt.After(end) || lastEventfulEntry.QueriedAt.Before(start) {
			// Session does not overlap with requested interval
			return false
		}
		return true
	}

	lastEventfulIndex := -1
	sessionStartIndex := -1

	consecutive := true

	for i := 0; i < len(stats); i++ {
		if sessionStartIndex == -1 {
			// Start a new session
			sessionStartIndex = i
			lastEventfulIndex = i
			consecutive = true
			continue
		}

		if lastEventfulIndex == -1 {
			err := fmt.Errorf("lastEventfulIndex is -1 in ComputeSessions")
			reporting.Report(ctx, err, map[string]string{
				"sessionStartIndex": fmt.Sprintf("%d", sessionStartIndex),
				"currentIndex":      fmt.Sprintf("%d", i),
				"statsLength":       fmt.Sprintf("%d", len(stats)),
			})
			return []domain.Session{}
		}

		stat := &stats[i]
		sessionStart := &stats[sessionStartIndex]
		lastEventfulEntry := &stats[lastEventfulIndex]

		// If no activity since session start, move session start to this
		startGamesPlayed, startExperience := getProgressStats(sessionStart)
		currentGamesPlayed, currentExperience := getProgressStats(stat)
		if currentGamesPlayed == startGamesPlayed && currentExperience == startExperience {
			sessionStartIndex = i
			lastEventfulIndex = i
			continue
		}

		// If more than 60 minutes since last activity, end session
		if stat.QueriedAt.Sub(lastEventfulEntry.QueriedAt) > 60*time.Minute {
			if includeSession(sessionStart, lastEventfulEntry) {
				sessions = append(sessions, domain.Session{
					Start:       *sessionStart,
					End:         *lastEventfulEntry,
					Consecutive: consecutive,
				})
			}
			// Jump back to right after the last eventful entry (loop adds one)
			// This makes sure we include any non-eventful trailing entries, as they could
			// be the start of a new session.
			// E.g. 1, 2, 3, 4, 4, 4, 5, 6, 7 - we don't want to skip over all the 4s and do
			// 1-4, 5-7, we want 1-4, 4-7
			i = lastEventfulIndex
			sessionStartIndex = -1
			lastEventfulIndex = -1
			continue
		}

		lastEventfulGamesPlayed, lastEventfulExperience := getProgressStats(lastEventfulEntry)

		// Games played decreased or increased by more than 2 -> not consecutive
		// NOTE: We allow an increase by 2 in case a player loses a game and that game finishes
		//       during the next game. This would cause an increase of 2 when the stats are queried
		//       at the end of the second game.
		//       This could cause a jump of more than 2 as well, but that is less likely to happen
		if currentGamesPlayed < lastEventfulGamesPlayed || currentGamesPlayed > lastEventfulGamesPlayed+2 {
			consecutive = false
		}

		// Stats changed
		if lastEventfulGamesPlayed != currentGamesPlayed || lastEventfulExperience != currentExperience {
			lastEventfulIndex = i
		}
	}

	// Add the last session if it was not added by the loop due to inactivity
	sessionStart := &stats[sessionStartIndex]
	lastEventfulEntry := &stats[lastEventfulIndex]

	if includeSession(sessionStart, lastEventfulEntry) {
		sessions = append(sessions, domain.Session{
			Start:       *sessionStart,
			End:         *lastEventfulEntry,
			Consecutive: consecutive,
		})
	}

	return sessions
}

type GetBestSessions = func(
	ctx context.Context,
	uuid string,
	start, end time.Time,
) (StatsByMetric, error)

// StatsByMetric holds the best session for each metric
type StatsByMetric struct {
	Playtime   domain.Session
	FinalKills domain.Session
	Wins       domain.Session
	FKDR       domain.Session
	Stars      domain.Session
}

func BuildGetBestSessions(getSessions GetSessions) GetBestSessions {
	return func(ctx context.Context,
		uuid string,
		start, end time.Time,
	) (StatsByMetric, error) {
		sessions, err := getSessions(ctx, uuid, start, end)
		if err != nil {
			return StatsByMetric{}, err
		}

		if len(sessions) == 0 {
			return StatsByMetric{}, domain.ErrNoSessions
		}

		// Temporary struct with pointers for efficient updates
		type bestSessionsPointers struct {
			Playtime   *domain.Session
			FinalKills *domain.Session
			Wins       *domain.Session
			FKDR       *domain.Session
			Stars      *domain.Session
		}

		// Initialize with first session
		best := bestSessionsPointers{
			Playtime:   &sessions[0],
			FinalKills: &sessions[0],
			Wins:       &sessions[0],
			FKDR:       &sessions[0],
			Stars:      &sessions[0],
		}

		// Iterate through remaining sessions
		for i := 1; i < len(sessions); i++ {
			session := &sessions[i]
			best.Playtime = getBest(best.Playtime, session, domain.Session.Playtime)
			best.FinalKills = getBest(best.FinalKills, session, domain.Session.FinalKills)
			best.Wins = getBest(best.Wins, session, domain.Session.Wins)
			best.FKDR = getBest(best.FKDR, session, domain.Session.FKDR)
			best.Stars = getBest(best.Stars, session, domain.Session.Stars)
		}

		// Convert to StatsByMetric
		return StatsByMetric{
			Playtime:   *best.Playtime,
			FinalKills: *best.FinalKills,
			Wins:       *best.Wins,
			FKDR:       *best.FKDR,
			Stars:      *best.Stars,
		}, nil
	}
}

// getBest returns the best session based on a comparison function that returns a cmp.Ordered value
func getBest[T cmp.Ordered](current *domain.Session, candidate *domain.Session, getValue func(domain.Session) T) *domain.Session {
	if getValue(*candidate) > getValue(*current) {
		return candidate
	}
	return current
}
