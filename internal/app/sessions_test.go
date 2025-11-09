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
		expectedBest   *domain.BestSessions
		expectedErrMsg string
		getSessionsErr error
	}{
		{
			name:         "empty sessions",
			sessions:     []domain.Session{},
			expectedBest: &domain.BestSessions{},
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
			expectedBest: &domain.BestSessions{
				Playtime:   nil, // Will be set to first session
				FinalKills: nil, // Will be set to first session
				Wins:       nil, // Will be set to first session
				FKDR:       nil, // Will be set to first session
				Stars:      nil, // Will be set to first session
			},
		},
		{
			name: "multiple sessions with different bests",
			sessions: []domain.Session{
				// Session with most playtime
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
				// Session with most final kills
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
				// Session with highest stars
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
			expectedBest: &domain.BestSessions{
				Playtime:   nil, // Will be set to session 0 (3 hours)
				FinalKills: nil, // Will be set to session 1 (20 final kills)
				Wins:       nil, // Will be set to session 0 (first with 0 wins)
				FKDR:       nil, // Will be set to session 1
				Stars:      nil, // Will be set to session 2 (highest end stars)
			},
		},
		{
			name:           "error from getSessions",
			sessions:       nil,
			getSessionsErr: assert.AnError,
			expectedErrMsg: "assert.AnError",
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
			require.NotNil(t, result)

			// For single session test, all best sessions should point to the same session
			if tt.name == "single session" {
				require.NotNil(t, result.Playtime)
				require.NotNil(t, result.FinalKills)
				require.NotNil(t, result.Wins)
				require.NotNil(t, result.FKDR)
				require.NotNil(t, result.Stars)
				// All should be the same session
				require.Equal(t, result.Playtime, result.FinalKills)
				require.Equal(t, result.Playtime, result.Wins)
				require.Equal(t, result.Playtime, result.FKDR)
				require.Equal(t, result.Playtime, result.Stars)
			}

			// For multiple sessions test, verify the correct bests
			if tt.name == "multiple sessions with different bests" {
				// Session 0 has most playtime (3 hours)
				require.NotNil(t, result.Playtime)
				require.Equal(t, 3*time.Hour, result.Playtime.End.QueriedAt.Sub(result.Playtime.Start.QueriedAt))

				// Session 1 has most final kills (20)
				require.NotNil(t, result.FinalKills)
				fkDiff := result.FinalKills.End.Overall.FinalKills - result.FinalKills.Start.Overall.FinalKills
				require.Equal(t, 20, fkDiff)

				// Session 2 has highest stars
				require.NotNil(t, result.Stars)
				require.Equal(t, int64(10000), result.Stars.End.Experience)
			}

			// For empty sessions
			if tt.name == "empty sessions" {
				require.Nil(t, result.Playtime)
				require.Nil(t, result.FinalKills)
				require.Nil(t, result.Wins)
				require.Nil(t, result.FKDR)
				require.Nil(t, result.Stars)
			}
		})
	}
}
