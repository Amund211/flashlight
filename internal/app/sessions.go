package app

import (
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
) (*domain.BestSessions, error)

func BuildGetBestSessions(getSessions GetSessions) GetBestSessions {
	return func(ctx context.Context,
		uuid string,
		start, end time.Time,
	) (*domain.BestSessions, error) {
		sessions, err := getSessions(ctx, uuid, start, end)
		if err != nil {
			return nil, err
		}

		if len(sessions) == 0 {
			return &domain.BestSessions{}, nil
		}

		bestSessions := &domain.BestSessions{}

		for i := range sessions {
			session := &sessions[i]

			// Calculate metrics for this session
			playtime := session.End.QueriedAt.Sub(session.Start.QueriedAt)
			finalKills := session.End.Overall.FinalKills - session.Start.Overall.FinalKills
			wins := session.End.Overall.Wins - session.Start.Overall.Wins
			stars := session.End.Stars()

			// FKDR calculation
			finalDeaths := session.End.Overall.FinalDeaths - session.Start.Overall.FinalDeaths
			var fkdr float64
			if finalDeaths > 0 {
				fkdr = float64(finalKills) / float64(finalDeaths)
			} else if finalKills > 0 {
				fkdr = float64(finalKills)
			} else {
				fkdr = 0
			}

			// Update best session for playtime
			if bestSessions.Playtime == nil {
				bestSessions.Playtime = session
			} else {
				bestPlaytime := bestSessions.Playtime.End.QueriedAt.Sub(bestSessions.Playtime.Start.QueriedAt)
				if playtime > bestPlaytime {
					bestSessions.Playtime = session
				}
			}

			// Update best session for final kills
			if bestSessions.FinalKills == nil {
				bestSessions.FinalKills = session
			} else {
				bestFinalKills := bestSessions.FinalKills.End.Overall.FinalKills - bestSessions.FinalKills.Start.Overall.FinalKills
				if finalKills > bestFinalKills {
					bestSessions.FinalKills = session
				}
			}

			// Update best session for wins
			if bestSessions.Wins == nil {
				bestSessions.Wins = session
			} else {
				bestWins := bestSessions.Wins.End.Overall.Wins - bestSessions.Wins.Start.Overall.Wins
				if wins > bestWins {
					bestSessions.Wins = session
				}
			}

			// Update best session for FKDR
			if bestSessions.FKDR == nil {
				bestSessions.FKDR = session
			} else {
				bestFinalKills := bestSessions.FKDR.End.Overall.FinalKills - bestSessions.FKDR.Start.Overall.FinalKills
				bestFinalDeaths := bestSessions.FKDR.End.Overall.FinalDeaths - bestSessions.FKDR.Start.Overall.FinalDeaths
				var bestFKDR float64
				if bestFinalDeaths > 0 {
					bestFKDR = float64(bestFinalKills) / float64(bestFinalDeaths)
				} else if bestFinalKills > 0 {
					bestFKDR = float64(bestFinalKills)
				} else {
					bestFKDR = 0
				}
				if fkdr > bestFKDR {
					bestSessions.FKDR = session
				}
			}

			// Update best session for stars
			if bestSessions.Stars == nil {
				bestSessions.Stars = session
			} else {
				bestStars := bestSessions.Stars.End.Stars()
				if stars > bestStars {
					bestSessions.Stars = session
				}
			}
		}

		return bestSessions, nil
	}
}
