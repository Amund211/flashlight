package app

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/Amund211/flashlight/internal/adapters/playerrepository"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

type GetSessions = func(
	ctx context.Context,
	uuid string,
	start, end time.Time,
) ([]domain.Session, error)

func BuildGetSessions(
	repo playerrepository.PlayerRepository,
	updatePlayerInInterval UpdatePlayerInInterval,
) GetSessions {
	return func(ctx context.Context,
		uuid string,
		start, end time.Time,
	) ([]domain.Session, error) {
		if !strutils.UUIDIsNormalized(uuid) {
			err := fmt.Errorf("UUID is not normalized")
			reporting.Report(ctx, err)
			return nil, err
		}

		if start.After(end) {
			err := fmt.Errorf("start time is after end time")
			reporting.Report(ctx, err)
			return nil, err
		}

		err := updatePlayerInInterval(ctx, uuid, start, end)
		if err != nil {
			// NOTE: UpdatePlayerInInterval implementations handle their own error reporting
			logging.FromContext(ctx).ErrorContext(ctx, "Failed to update player data in interval", "error", err)

			// NOTE: We continue even though we failed to update player data
			// We may still be able to get the history and fulfill the request
		}

		sessions, err := repo.GetSessions(ctx, uuid, start, end)
		if err != nil {
			// NOTE: PlayerRepository implementations handle their own error reporting
			return nil, fmt.Errorf("failed to get sessions: %w", err)
		}

		return sessions, nil
	}
}

type GetBestSessions = func(
	ctx context.Context,
	uuid string,
	start, end time.Time,
) (domain.BestSessions, error)

func BuildGetBestSessions(getSessions GetSessions) GetBestSessions {
	return func(ctx context.Context,
		uuid string,
		start, end time.Time,
	) (domain.BestSessions, error) {
		sessions, err := getSessions(ctx, uuid, start, end)
		if err != nil {
			return domain.BestSessions{}, err
		}

		if len(sessions) == 0 {
			return domain.BestSessions{}, domain.ErrNoSessions
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

		// Convert to domain.BestSessions
		return domain.BestSessions{
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
