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

type mockMostRecentRepository struct {
	pit *domain.PlayerPIT
	err error
}

func (m *mockMostRecentRepository) GetMostRecentPlayerPIT(ctx context.Context, playerUUID string) (*domain.PlayerPIT, error) {
	return m.pit, m.err
}

func TestBuildGetLatestSession(t *testing.T) {
	t.Parallel()

	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	lastQueriedAt := time.Date(2024, 6, 15, 20, 30, 0, 0, time.UTC)

	t.Run("anchors GetSessionAt at the most recent stat's time and returns its result", func(t *testing.T) {
		t.Parallel()

		mostRecent := domaintest.NewPlayerBuilder(uuid).FromDB().Build(lastQueriedAt)
		repo := &mockMostRecentRepository{pit: &mostRecent}

		startPIT := domaintest.NewPlayerBuilder(uuid).FromDB().Fours().WithGamesPlayed(10).Build(lastQueriedAt.Add(-time.Hour))
		want := app.SessionAtResult{
			Session: &domain.Session{Start: startPIT, End: mostRecent, Consecutive: true},
			Games:   []app.GameSegment{{Start: startPIT, End: mostRecent, Game: nil}},
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

		getLatestSession := app.BuildGetLatestSession(repo, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.True(t, called)
		require.Equal(t, uuid, gotUUID)
		require.WithinDuration(t, lastQueriedAt, gotAt, 0)
		require.Equal(t, want, got)
	})

	t.Run("runs the real GetSessionAt pipeline end-to-end", func(t *testing.T) {
		t.Parallel()

		// Wire the real GetSessionAt — real BuildComputeSessions over a
		// fixed PIT slice — instead of a mock, so BuildGetLatestSession
		// exercises the actual bracket-matching and game derivation against
		// ComputeSessions. Mirrors session_at_test.go.
		nowFarFuture := func() time.Time { return lastQueriedAt.Add(365 * 24 * time.Hour) }
		computeSessions := app.BuildComputeSessions(nowFarFuture)
		fixedStats := func(stats []domain.PlayerPIT) app.GetPlayerPITs {
			return func(ctx context.Context, _ string, _, _ time.Time) ([]domain.PlayerPIT, error) {
				return stats, nil
			}
		}

		// Three doubles games over four snapshots. The most recent snapshot
		// (p3) is what GetMostRecentPlayerPIT returns and where GetSessionAt
		// gets anchored, so the computed session must bracket exactly that
		// time and end on it.
		b := domaintest.NewPlayerBuilder(uuid).
			WithExperience(1000).FromDB().
			Doubles().
			WithGamesPlayed(10).WithWins(5).WithLosses(5).
			WithBedsBroken(4).WithBedsLost(3).
			WithFinalKills(20).WithFinalDeaths(10).
			WithKills(50).WithDeaths(30)
		p0 := b.Build(lastQueriedAt.Add(-45 * time.Minute))

		// doubles +1, won
		p1 := b.
			WithExperience(1300).
			Doubles().
			WithGamesPlayed(11).WithWins(6).
			WithBedsBroken(5).
			WithFinalKills(24).
			WithKills(58).WithDeaths(32).Build(lastQueriedAt.Add(-30 * time.Minute))

		// doubles +1, lost, final-died, bed-lost
		p2 := b.
			WithExperience(1500).
			Doubles().
			WithGamesPlayed(12).WithLosses(6).
			WithBedsLost(4).
			WithFinalKills(26).WithFinalDeaths(11).
			WithKills(62).WithDeaths(36).Build(lastQueriedAt.Add(-15 * time.Minute))

		// doubles +1, won — most recent stat
		p3 := b.
			WithExperience(1800).
			Doubles().
			WithGamesPlayed(13).WithWins(7).
			WithBedsBroken(6).
			WithFinalKills(30).
			WithKills(70).WithDeaths(38).Build(lastQueriedAt)

		repo := &mockMostRecentRepository{pit: &p3}
		getSessionAt := app.BuildGetSessionAt(
			fixedStats([]domain.PlayerPIT{p0, p1, p2, p3}),
			computeSessions,
		)

		getLatestSession := app.BuildGetLatestSession(repo, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.Equal(t, app.SessionAtResult{
			Session: &domain.Session{Start: p0, End: p3, Consecutive: true},
			Games: []app.GameSegment{
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
				{Start: p2, End: p3, Game: &domain.GameResult{
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
			},
		}, got)
	})

	t.Run("returns empty result and skips GetSessionAt when the player has no stats", func(t *testing.T) {
		t.Parallel()

		repo := &mockMostRecentRepository{err: domain.ErrPlayerNotFound}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called when the player has no stats")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, getSessionAt)

		got, err := getLatestSession(t.Context(), uuid)
		require.NoError(t, err)
		require.Equal(t, app.SessionAtResult{}, got)
	})

	t.Run("propagates repository errors and skips GetSessionAt", func(t *testing.T) {
		t.Parallel()

		repo := &mockMostRecentRepository{err: errors.New("boom")}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called when the repository fails")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, getSessionAt)

		_, err := getLatestSession(t.Context(), uuid)
		require.Error(t, err)
	})

	t.Run("rejects a non-normalized uuid without touching the repository", func(t *testing.T) {
		t.Parallel()

		repo := &mockMostRecentRepository{err: errors.New("repo should not be queried")}

		getSessionAt := func(ctx context.Context, u string, at time.Time) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getSessionAt should not be called for an invalid uuid")
			return app.SessionAtResult{}, nil
		}

		getLatestSession := app.BuildGetLatestSession(repo, getSessionAt)

		_, err := getLatestSession(t.Context(), "not-normalized")
		require.Error(t, err)
	})
}
