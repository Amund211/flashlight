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

	singleSession := domain.Session{
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
	}

	multiSession0 := domain.Session{
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
	}

	multiSession1 := domain.Session{
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
	}

	multiSession2 := domain.Session{
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
	}

	compSession0 := domain.Session{
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
	}

	compSession1 := domain.Session{
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
	}

	compSession2 := domain.Session{
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
	}

	tests := []struct {
		name           string
		sessions       []domain.Session
		expectedResult app.StatsByMetric
		expectedErrMsg string
		getSessionsErr error
	}{
		{
			name:           "empty sessions",
			sessions:       []domain.Session{},
			expectedResult: app.StatsByMetric{},
			expectedErrMsg: "no sessions found",
		},
		{
			name:     "single session",
			sessions: []domain.Session{singleSession},
			expectedResult: app.StatsByMetric{
				Playtime:   singleSession,
				FinalKills: singleSession,
				Wins:       singleSession,
				FKDR:       singleSession,
				Stars:      singleSession,
			},
		},
		{
			name:     "multiple sessions with different bests",
			sessions: []domain.Session{multiSession0, multiSession1, multiSession2},
			expectedResult: app.StatsByMetric{
				Playtime:   multiSession0, // 3 hours
				FinalKills: multiSession1, // 20 final kills
				Wins:       multiSession0, // 0 wins (first with 0)
				FKDR:       multiSession1, // 20/0 = infinite (treated as 20)
				Stars:      multiSession2, // 9300 exp gain
			},
		},
		{
			name:           "error from getSessions",
			sessions:       nil,
			getSessionsErr: assert.AnError,
			expectedResult: app.StatsByMetric{},
			expectedErrMsg: "assert.AnError",
		},
		{
			name:     "comprehensive metric test",
			sessions: []domain.Session{compSession0, compSession1, compSession2},
			expectedResult: app.StatsByMetric{
				Playtime:   compSession0, // 5 hours
				FinalKills: compSession1, // 50 final kills
				Wins:       compSession1, // 10 wins
				FKDR:       compSession2, // 20/1 = 20.0
				Stars:      compSession2, // 49300 exp gain
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
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
