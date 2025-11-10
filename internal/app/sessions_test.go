package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/adapters/playerrepository"
	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSessionsRepository struct {
	playerrepository.StubPlayerRepository
	sessions []domain.Session
	err      error
}

func (m *mockSessionsRepository) GetSessions(ctx context.Context, playerUUID string, start, end time.Time) ([]domain.Session, error) {
	return m.sessions, m.err
}

func newMockSessionsRepository(t *testing.T, sessions []domain.Session, err error) *mockSessionsRepository {
	if err == nil {
		require.NotNil(t, sessions)
	} else {
		require.Nil(t, sessions)
	}

	return &mockSessionsRepository{
		sessions: sessions,
		err:      err,
	}
}

func TestBuildGetSessions(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("now within range", func(t *testing.T) {
		t.Parallel()

		uuid := "12345678-1234-1234-1234-123456789012"

		timeCases := []struct {
			name  string
			start time.Time
			end   time.Time
		}{
			{
				name:  "-1hr, +1hr",
				start: now.Add(-1 * time.Hour),
				end:   now.Add(1 * time.Hour),
			},
			{
				name:  "-0hr, +1hr",
				start: now,
				end:   now.Add(1 * time.Hour),
			},
			{
				name:  "-1hr, +0hr",
				start: now.Add(-1 * time.Hour),
				end:   now,
			},
			{
				name:  "-1dy, +1dy",
				start: now.Add(-24 * time.Hour),
				end:   now.Add(24 * time.Hour),
			},
		}

		sessionsCases := []struct {
			name     string
			sessions []domain.Session
		}{
			{
				name:     "empty sessions",
				sessions: []domain.Session{},
			},
			{
				name: "non-empty sessions",
				sessions: []domain.Session{
					{
						Start:       domaintest.NewPlayerBuilder(uuid, now).WithExperience(500).Build(),
						End:         domaintest.NewPlayerBuilder(uuid, now).WithExperience(501).Build(),
						Consecutive: true,
					},
				},
			},
		}

		for _, timeCase := range timeCases {
			t.Run(timeCase.name, func(t *testing.T) {
				t.Parallel()
				for _, sessionsCase := range sessionsCases {
					t.Run(sessionsCase.name, func(t *testing.T) {
						t.Parallel()

						updatePlayerInIntervalCalled := false
						updatePlayerInInterval := func(ctx context.Context, updateUUID string, start, end time.Time) error {
							t.Helper()
							require.Equal(t, uuid, updateUUID)
							require.WithinDuration(t, timeCase.start, start, 0)
							require.WithinDuration(t, timeCase.end, end, 0)
							updatePlayerInIntervalCalled = true
							return nil
						}

						getSessions := app.BuildGetSessions(
							newMockSessionsRepository(t, sessionsCase.sessions, nil),
							updatePlayerInInterval,
						)

						sessions, err := getSessions(t.Context(), uuid, timeCase.start, timeCase.end)
						require.NoError(t, err)
						require.Equal(t, sessionsCase.sessions, sessions)

						require.True(t, updatePlayerInIntervalCalled)
					})
				}
			})
		}
	})

	t.Run("now outside range", func(t *testing.T) {
		t.Parallel()

		uuid := "12345678-1234-1234-1234-123456789012"

		timeCases := []struct {
			name  string
			start time.Time
			end   time.Time
		}{
			{
				name:  "-2hr, -1hr",
				start: now.Add(-2 * time.Hour),
				end:   now.Add(-1 * time.Hour),
			},
			{
				name:  "+1ms, +1hr",
				start: now.Add(1 * time.Millisecond),
				end:   now.Add(1 * time.Hour),
			},
			{
				name:  "-1hr, -1s",
				start: now.Add(-1 * time.Hour),
				end:   now.Add(-1 * time.Second),
			},
			{
				name:  "-30dy, -15dy",
				start: now.Add(-30 * 24 * time.Hour),
				end:   now.Add(-15 * 24 * time.Hour),
			},
		}

		sessionsCases := []struct {
			name     string
			sessions []domain.Session
		}{
			{
				name:     "empty sessions",
				sessions: []domain.Session{},
			},
			{
				name: "non-empty sessions",
				sessions: []domain.Session{
					{
						Start:       domaintest.NewPlayerBuilder(uuid, now).WithExperience(500).Build(),
						End:         domaintest.NewPlayerBuilder(uuid, now).WithExperience(501).Build(),
						Consecutive: true,
					},
				},
			},
		}

		for _, timeCase := range timeCases {
			t.Run(timeCase.name, func(t *testing.T) {
				t.Parallel()
				for _, sessionsCase := range sessionsCases {
					t.Run(sessionsCase.name, func(t *testing.T) {
						t.Parallel()

						updatePlayerInIntervalCalled := false
						updatePlayerInInterval := func(ctx context.Context, updateUUID string, start, end time.Time) error {
							t.Helper()
							require.Equal(t, uuid, updateUUID)
							require.WithinDuration(t, timeCase.start, start, 0)
							require.WithinDuration(t, timeCase.end, end, 0)
							updatePlayerInIntervalCalled = true
							return nil
						}

						getSessions := app.BuildGetSessions(
							newMockSessionsRepository(t, sessionsCase.sessions, nil),
							updatePlayerInInterval,
						)

						sessions, err := getSessions(t.Context(), uuid, timeCase.start, timeCase.end)
						require.NoError(t, err)
						require.Equal(t, sessionsCase.sessions, sessions)

						require.True(t, updatePlayerInIntervalCalled)
					})
				}
			})
		}
	})

	t.Run("update player in interval fails", func(t *testing.T) {
		t.Parallel()

		uuid := "12345678-1234-1234-1234-123456789012"

		start := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2024, time.March, 31, 23, 59, 59, 999_999_999, time.UTC)

		expectedSessions := []domain.Session{
			{
				Start:       domaintest.NewPlayerBuilder(uuid, now).WithExperience(500).Build(),
				End:         domaintest.NewPlayerBuilder(uuid, now).WithExperience(501).Build(),
				Consecutive: true,
			},
		}

		updatePlayerInIntervalCalled := false
		updatePlayerInInterval := func(ctx context.Context, updateUUID string, updateStart, updateEnd time.Time) error {
			t.Helper()
			require.Equal(t, uuid, updateUUID)
			require.WithinDuration(t, start, updateStart, 0)
			require.WithinDuration(t, end, updateEnd, 0)
			updatePlayerInIntervalCalled = true
			return assert.AnError
		}

		getSessions := app.BuildGetSessions(
			newMockSessionsRepository(t, expectedSessions, nil),
			updatePlayerInInterval,
		)

		sessions, err := getSessions(t.Context(), uuid, start, end)
		// Should not error even if updatePlayerInInterval fails
		require.NoError(t, err)
		require.Equal(t, expectedSessions, sessions)

		require.True(t, updatePlayerInIntervalCalled)
	})
}

func TestBuildGetBestSessions(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	tests := []struct {
		name           string
		sessions       []domain.Session
		expectedErrMsg string
		getSessionsErr error
	}{
		{
			name:           "empty sessions",
			sessions:       []domain.Session{},
			expectedErrMsg: "no sessions found",
		},
		{
			name: "single session",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(uuid, now).
						WithExperience(500).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(10).
							WithFinalKills(20).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
						WithExperience(1000).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(15).
							WithFinalKills(30).
							Build()).
						Build(),
					Consecutive: true,
				},
			},
		},
		{
			name: "multiple sessions with different bests",
			sessions: []domain.Session{
				// Session 0: Best playtime (3 hours)
				{
					Start: domaintest.NewPlayerBuilder(uuid, now).
						WithExperience(500).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(10).
							WithFinalKills(20).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(3*time.Hour)).
						WithExperience(600).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(11).
							WithFinalKills(21).
							Build()).
						Build(),
					Consecutive: true,
				},
				// Session 1: Best final kills (20)
				{
					Start: domaintest.NewPlayerBuilder(uuid, now.Add(4*time.Hour)).
						WithExperience(600).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(11).
							WithFinalKills(21).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(5*time.Hour)).
						WithExperience(700).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(16).
							WithFinalKills(41).
							Build()).
						Build(),
					Consecutive: true,
				},
				// Session 2: Best stars delta (9300 exp gain)
				{
					Start: domaintest.NewPlayerBuilder(uuid, now.Add(6*time.Hour)).
						WithExperience(700).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(16).
							WithFinalKills(41).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(7*time.Hour)).
						WithExperience(10000).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(17).
							WithFinalKills(42).
							Build()).
						Build(),
					Consecutive: true,
				},
			},
		},
		{
			name:           "error from getSessions",
			sessions:       nil,
			getSessionsErr: assert.AnError,
			expectedErrMsg: "assert.AnError",
		},
		{
			name: "comprehensive metric test",
			sessions: []domain.Session{
				// Session 0: Best playtime (5 hours)
				{
					Start: domaintest.NewPlayerBuilder(uuid, now).
						WithExperience(500).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(10).
							WithFinalKills(100).
							WithFinalDeaths(50).
							WithWins(5).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(5*time.Hour)).
						WithExperience(600).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(15).
							WithFinalKills(110).
							WithFinalDeaths(60).
							WithWins(7).
							Build()).
						Build(),
					Consecutive: true,
				},
				// Session 1: Best final kills (50) and best wins (10)
				{
					Start: domaintest.NewPlayerBuilder(uuid, now.Add(6*time.Hour)).
						WithExperience(600).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(15).
							WithFinalKills(110).
							WithFinalDeaths(60).
							WithWins(7).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(7*time.Hour)).
						WithExperience(700).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(25).
							WithFinalKills(160).
							WithFinalDeaths(65).
							WithWins(17).
							Build()).
						Build(),
					Consecutive: true,
				},
				// Session 2: Best FKDR (20/1 = 20.0) and best stars delta (49300 exp gain)
				{
					Start: domaintest.NewPlayerBuilder(uuid, now.Add(8*time.Hour)).
						WithExperience(700).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(25).
							WithFinalKills(160).
							WithFinalDeaths(65).
							WithWins(17).
							Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(uuid, now.Add(9*time.Hour)).
						WithExperience(50000).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithGamesPlayed(30).
							WithFinalKills(180).
							WithFinalDeaths(66).
							WithWins(20).
							Build()).
						Build(),
					Consecutive: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mock getSessions
			getSessions := func(ctx context.Context, uuid string, start, end time.Time) ([]domain.Session, error) {
				if tt.getSessionsErr != nil {
					return nil, tt.getSessionsErr
				}
				return tt.sessions, nil
			}

			getBestSessions := app.BuildGetBestSessions(getSessions)

			start := now.Add(-1 * time.Hour)
			end := now.Add(10 * time.Hour)

			result, err := getBestSessions(t.Context(), uuid, start, end)

			if tt.expectedErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// For single session test, all best sessions should be the same session
			if tt.name == "single session" {
				// All should be the same session
				require.Equal(t, result.Playtime, result.FinalKills)
				require.Equal(t, result.Playtime, result.Wins)
				require.Equal(t, result.Playtime, result.FKDR)
				require.Equal(t, result.Playtime, result.Stars)
			}

			// For multiple sessions test, verify the correct bests
			if tt.name == "multiple sessions with different bests" {
				// Session 0 has most playtime (3 hours)
				require.Equal(t, 3*time.Hour, result.Playtime.End.QueriedAt.Sub(result.Playtime.Start.QueriedAt))

				// Session 1 has most final kills (20)
				fkDiff := result.FinalKills.End.Overall.FinalKills - result.FinalKills.Start.Overall.FinalKills
				require.Equal(t, 20, fkDiff)

				// Session 2 has highest stars delta (9300 exp gain)
				starsDiff := result.Stars.End.Experience - result.Stars.Start.Experience
				require.Equal(t, int64(9300), starsDiff)
			}

			// For comprehensive metric test
			if tt.name == "comprehensive metric test" {
				// Session 0 has best playtime (5 hours)
				require.Equal(t, 5*time.Hour, result.Playtime.End.QueriedAt.Sub(result.Playtime.Start.QueriedAt))

				// Session 1 has best final kills (50)
				fkDiff := result.FinalKills.End.Overall.FinalKills - result.FinalKills.Start.Overall.FinalKills
				require.Equal(t, 50, fkDiff)

				// Session 1 has best wins (10)
				winsDiff := result.Wins.End.Overall.Wins - result.Wins.Start.Overall.Wins
				require.Equal(t, 10, winsDiff)

				// Session 2 has best FKDR (20/1 = 20.0)
				fkdrFk := result.FKDR.End.Overall.FinalKills - result.FKDR.Start.Overall.FinalKills
				fkdrFd := result.FKDR.End.Overall.FinalDeaths - result.FKDR.Start.Overall.FinalDeaths
				fkdr := float64(fkdrFk) / float64(fkdrFd)
				require.InDelta(t, 20.0, fkdr, 0.01)

				// Session 2 has best stars delta (49300 exp gain)
				starsDiff := result.Stars.End.Experience - result.Stars.Start.Experience
				require.Equal(t, int64(49300), starsDiff)
			}
		})
	}
}
