package app

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

// latestSessionDiscoveryLimit bounds how many of the player's most recent
// stats we scan to locate their latest session.
//
// It must comfortably exceed the number of trailing non-eventful "still-seen"
// pings plus the stats making up the session itself. StorePlayer only dedups
// stats within a trailing 1h window, so an inactive-but-watched player accrues
// at most ~one duplicate ping per hour they're viewed; a few hundred rows
// therefore covers weeks of such pings. If a real session sits entirely beyond
// this many rows we won't find it — see the report below.
const latestSessionDiscoveryLimit = 300

type latestSessionPlayerRepository interface {
	GetRecentPlayerPITs(ctx context.Context, playerUUID string, limit int) ([]domain.PlayerPIT, error)
}

type GetLatestSession = func(
	ctx context.Context,
	uuid string,
) (SessionAtResult, error)

// BuildGetLatestSession constructs a GetLatestSession that returns the player's
// most recent session, in the exact same shape as GetSessionAt.
//
// It runs a cheap read-only "discovery" pass over the player's most recent
// stats, computes their sessions, and takes the latest one. It then delegates
// to GetSessionAt anchored at that session's End. GetSessionAt does the rest:
// it (re)fetches and refreshes the ±24h window around that time, recomputes the
// session bracketing it, and derives the game segments.
//
// Anchoring at the last session's End — rather than at the most recently stored
// stat — is deliberate. An inactive player re-viewed more than 1h after their
// last game gets a fresh duplicate stat appended (StorePlayer only dedups
// within a trailing 1h window). That trailing stat is not part of any session,
// so anchoring GetSessionAt there would look past the real session's end and
// return "no session". The discovery pass sidesteps this: non-eventful trailing
// pings never extend a session, so the computed latest session ends on the last
// stat that actually saw activity.
//
// If the player has no session in the scanned window, an empty SessionAtResult
// (nil session) is returned, mirroring GetSessionAt's behaviour when no session
// overlaps the requested time.
func BuildGetLatestSession(
	repo latestSessionPlayerRepository,
	computeSessions ComputeSessions,
	getSessionAt GetSessionAt,
) GetLatestSession {
	return func(ctx context.Context, uuid string) (SessionAtResult, error) {
		if !strutils.UUIDIsNormalized(uuid) {
			err := fmt.Errorf("UUID is not normalized")
			reporting.Report(ctx, err)
			return SessionAtResult{}, err
		}

		// Discovery pass: a pure read (no refresh) used only to locate the
		// anchor for GetSessionAt.
		recent, err := repo.GetRecentPlayerPITs(ctx, uuid, latestSessionDiscoveryLimit)
		if err != nil {
			// NOTE: PlayerRepository implementations handle their own error reporting
			return SessionAtResult{}, fmt.Errorf("failed to get recent player pits: %w", err)
		}

		if len(recent) < 2 {
			// Need at least two stats to form a session.
			return SessionAtResult{Session: nil, Games: nil}, nil
		}

		// recent is ascending, so these bounds span the whole scanned range and
		// ComputeSessions includes every session it finds.
		sessions := computeSessions(ctx, recent, recent[0].QueriedAt, recent[len(recent)-1].QueriedAt)
		if len(sessions) == 0 {
			if len(recent) == latestSessionDiscoveryLimit {
				// We scanned the full cap and still found no session: a real but
				// older session may sit just beyond it. Surface it rather than
				// silently returning "no session".
				reporting.Report(ctx,
					fmt.Errorf("no session found within latest-session discovery limit"),
					map[string]string{
						"limit": strconv.Itoa(latestSessionDiscoveryLimit),
					},
				)
			}
			return SessionAtResult{Session: nil, Games: nil}, nil
		}

		// ComputeSessions returns sessions in ascending order, so the last is the
		// most recent. Its End is an eventful stat, so it's guaranteed to fall
		// inside GetSessionAt's bracket.
		latest := sessions[len(sessions)-1]
		return getSessionAt(ctx, uuid, latest.End.QueriedAt)
	}
}
