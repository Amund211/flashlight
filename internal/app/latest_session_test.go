package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
)

type mockRecentRepository struct {
	pits     []domain.PlayerPIT
	err      error
	gotLimit int
	called   bool
}

func (m *mockRecentRepository) GetRecentPlayerPITs(ctx context.Context, playerUUID string, limit int) ([]domain.PlayerPIT, error) {
	m.called = true
	m.gotLimit = limit
	return m.pits, m.err
}

func TestBuildGetLatestSession(t *testing.T) {
	t.Parallel()

	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	lastQueriedAt := time.Date(2024, 6, 15, 20, 30, 0, 0, time.UTC)

	// nowFarFuture keeps the computed sessions from being marked Ongoing so the
	// tests assert on stable, completed sessions.
	nowFarFuture := func() time.Time { return lastQueriedAt.Add(365 * 24 * time.Hour) }

	// twoGameSession builds a small consecutive doubles session (p0..p2) plus,
	// when trailing is true, a non-eventful duplicate of p2 appended 2h later —
	// the kind of "still-seen" ping StorePlayer records for an inactive player
	// re-viewed more than 1h after their last game. Returns the stat slice, the
	// session's first stat, and the session's last eventful stat.
	buildSessionStats := func(trailing bool) (stats []domain.PlayerPIT, start, end domain.PlayerPIT) {
		b := domaintest.NewPlayerBuilder(uuid).
			WithExperience(1000).FromDB().
			Doubles().
			WithGamesPlayed(10).WithWins(5).WithLosses(5).
			WithBedsBroken(4).WithBedsLost(3).
			WithFinalKills(20).WithFinalDeaths(10).
			WithKills(50).WithDeaths(30)
		p0 := b.Build(lastQueriedAt.Add(-30 * time.Minute))

		// doubles +1, won
		p1 := b.
			WithExperience(1300).
			Doubles().
			WithGamesPlayed(11).WithWins(6).
			WithBedsBroken(5).
			WithFinalKills(24).
			WithKills(58).WithDeaths(32).Build(lastQueriedAt.Add(-15 * time.Minute))

		// doubles +1, lost, final-died, bed-lost — the last eventful stat
		p2 := b.
			WithExperience(1500).
			Doubles().
			WithGamesPlayed(12).WithLosses(6).
			WithBedsLost(4).
			WithFinalKills(26).WithFinalDeaths(11).
			WithKills(62).WithDeaths(36).Build(lastQueriedAt)

		stats = []domain.PlayerPIT{p0, p1, p2}
		if trailing {
			// Identical stats to p2, but queried 2h later: a trailing duplicate.
			p3 := b.Build(lastQueriedAt.Add(2 * time.Hour))
			stats = append(stats, p3)
		}
		return stats, p0, p2
	}

	wantGames := func(p0, p1, p2 domain.PlayerPIT) []app.GameSegment {
		return []app.GameSegment{
			{Start: p0, End: p1, Game: &domain.GameResult{
				Gamemode:   domain.GamemodeDoubles,
				Outcome:    domain.GameOutcomeWin,
				FinalKills: 4,
				FinalDeath: false,
				BedsBroken: 1,
				BedLost:    false,
				Kills:      8,
				Deaths:     2,
				Experience: 300,
			}},
			{Start: p1, End: p2, Game: &domain.GameResult{
				Gamemode:   domain.GamemodeDoubles,
				Outcome:    domain.GameOutcomeLoss,
				FinalKills: 2,
				FinalDeath: true,
				BedsBroken: 0,
				BedLost:    true,
				Kills:      4,
				Deaths:     4,
				Experience: 200,
			}},
		}
	}

	t.Run("anchors GetSessionAt at the latest session's end and returns its result", func(t *testing.T) {
		t.Parallel()

		stats, start, end := buildSessionStats(false)
		computeSessions := app.BuildComputeSessions(nowFarFuture)
		repo := &mockRecentRepository{pits: stats}

		want := app.SessionAtResult{
			Session: &domain.Session{Start: start, End: end, Consecutive: true},
			Games:   []app.GameSegment{{Start: start, End: end, Game: nil}},
		}

		var gotUUID string
		var gotAt time.Time
		called := false
		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			called = true
			gotUUID = u
			gotAt = at
			return want, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.True(t, called)
		require.Equal(t, uuid, gotUUID)
		// Anchored at the session's last eventful stat, not some other time.
		require.WithinDuration(t, end.QueriedAt, gotAt, 0)
		// Discovery scans a bounded, non-trivial number of recent stats.
		require.Positive(t, repo.gotLimit)
		require.LessOrEqual(t, repo.gotLimit, 1000)
		require.Equal(t, want, got)
	})

	t.Run("runs the real GetSessionAt pipeline end-to-end", func(t *testing.T) {
		t.Parallel()

		stats, _, _ := buildSessionStats(false)
		p0, p1, p2 := stats[0], stats[1], stats[2]

		computeSessions := app.BuildComputeSessions(nowFarFuture)
		fixedStats := func(ctx context.Context, _ string, _, _ time.Time) ([]domain.PlayerPIT, error) {
			return stats, nil
		}
		repo := &mockRecentRepository{pits: stats}
		getSessionAt := app.BuildGetSessionAt(fixedStats, computeSessions)

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.Equal(t, app.SessionAtResult{
			Session: &domain.Session{Start: p0, End: p2, Consecutive: true},
			Games:   wantGames(p0, p1, p2),
		}, got)
	})

	t.Run("finds the real session despite a trailing non-eventful duplicate stat", func(t *testing.T) {
		t.Parallel()

		// The most recent stat (p3) is a duplicate ping queried 2h after the
		// last game, so anchoring at it would return "no session". The discovery
		// pass must instead anchor at the session's real end (p2).
		stats, _, _ := buildSessionStats(true)
		p0, p1, p2 := stats[0], stats[1], stats[2]

		computeSessions := app.BuildComputeSessions(nowFarFuture)
		fixedStats := func(ctx context.Context, _ string, _, _ time.Time) ([]domain.PlayerPIT, error) {
			return stats, nil
		}
		repo := &mockRecentRepository{pits: stats}
		getSessionAt := app.BuildGetSessionAt(fixedStats, computeSessions)

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.Equal(t, app.SessionAtResult{
			Session: &domain.Session{Start: p0, End: p2, Consecutive: true},
			Games:   wantGames(p0, p1, p2),
		}, got)
	})

	t.Run("returns empty result and skips GetSessionAt when the player has no stats", func(t *testing.T) {
		t.Parallel()

		computeSessions := app.BuildComputeSessions(nowFarFuture)
		repo := &mockRecentRepository{pits: []domain.PlayerPIT{}}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called when the player has no stats")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.Equal(t, app.SessionAtResult{}, got)
	})

	t.Run("returns empty result and skips GetSessionAt when no session is found", func(t *testing.T) {
		t.Parallel()

		// Two stats, but they only differ in queried_at (an inactive player
		// pinged twice) — no activity, so no session.
		p0 := domaintest.NewPlayerBuilder(uuid).FromDB().Fours().WithGamesPlayed(10).Build(lastQueriedAt.Add(-3 * time.Hour))
		p1 := domaintest.NewPlayerBuilder(uuid).FromDB().Fours().WithGamesPlayed(10).Build(lastQueriedAt)

		computeSessions := app.BuildComputeSessions(nowFarFuture)
		repo := &mockRecentRepository{pits: []domain.PlayerPIT{p0, p1}}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called when no session is found")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.Equal(t, app.SessionAtResult{}, got)
	})

	t.Run("propagates repository errors and skips GetSessionAt", func(t *testing.T) {
		t.Parallel()

		computeSessions := app.BuildComputeSessions(nowFarFuture)
		repo := &mockRecentRepository{err: errors.New("boom")}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called when the repository fails")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		_, err := getLatestSession(t.Context(), uuid)
		require.Error(t, err)
	})

	t.Run("rejects a non-normalized uuid without touching the repository", func(t *testing.T) {
		t.Parallel()

		computeSessions := app.BuildComputeSessions(nowFarFuture)
		repo := &mockRecentRepository{err: errors.New("repo should not be queried")}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called for an invalid uuid")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, computeSessions, getSessionAt)

		_, err := getLatestSession(t.Context(), "not-normalized")
		require.Error(t, err)
		require.False(t, repo.called)
	})
}
