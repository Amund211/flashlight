package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

type latestSessionPlayerRepository interface {
	GetMostRecentPlayerPIT(ctx context.Context, playerUUID string) (*domain.PlayerPIT, error)
}

type GetLatestSession = func(
	ctx context.Context,
	uuid string,
) (SessionAtResult, error)

// BuildGetLatestSession constructs a GetLatestSession that returns the player's
// most recent session, in the exact same shape as GetSessionAt.
//
// It runs a cheap preliminary query for the timestamp of the player's most
// recently stored stat, then delegates to GetSessionAt anchored at that time.
// GetSessionAt does the rest: it fetches (and refreshes) the ±24h window around
// that time, computes the session bracketing it, and derives the game segments.
//
// If the player has no stored stats, an empty SessionAtResult (nil session) is
// returned, mirroring GetSessionAt's behaviour when no session overlaps the
// requested time.
func BuildGetLatestSession(
	repo latestSessionPlayerRepository,
	getSessionAt GetSessionAt,
) GetLatestSession {
	return func(ctx context.Context, uuid string) (SessionAtResult, error) {
		if !strutils.UUIDIsNormalized(uuid) {
			err := fmt.Errorf("UUID is not normalized")
			reporting.Report(ctx, err)
			return SessionAtResult{}, err
		}

		mostRecent, err := repo.GetMostRecentPlayerPIT(ctx, uuid)
		if errors.Is(err, domain.ErrPlayerNotFound) {
			// No stored stats -> no session. Same shape as GetSessionAt when
			// no session overlaps the requested time.
			return SessionAtResult{Session: nil, Games: nil}, nil
		}
		if err != nil {
			// NOTE: PlayerRepository implementations handle their own error reporting
			return SessionAtResult{}, fmt.Errorf("failed to get most recent player pit: %w", err)
		}

		return getSessionAt(ctx, uuid, mostRecent.QueriedAt)
	}
}
