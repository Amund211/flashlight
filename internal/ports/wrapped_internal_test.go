package ports

import (
	"context"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/require"
)

func TestComputeSessionLengths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name     string
		sessions []domain.Session
		want     *sessionLengthStats
	}{
		{
			name:     "empty sessions",
			sessions: []domain.Session{},
			want:     nil,
		},
		{
			name: "single session",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).Build(),
				},
			},
			want: &sessionLengthStats{
				Total:    2.0,
				Longest:  2.0,
				Shortest: 2.0,
				Average:  2.0,
			},
		},
		{
			name: "multiple sessions",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 18, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 3, 9, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 3, 10, 0, 0, 0, time.UTC)).Build(),
				},
			},
			want: &sessionLengthStats{
				Total:    7.0,
				Longest:  4.0,
				Shortest: 1.0,
				Average:  7.0 / 3.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeSessionLengths(ctx, tt.sessions)
			if tt.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tt.want.Total, got.Total)
				require.Equal(t, tt.want.Longest, got.Longest)
				require.Equal(t, tt.want.Shortest, got.Shortest)
				require.InDelta(t, tt.want.Average, got.Average, 0.0001)
			}
		})
	}
}

func TestComputeSessionsPerMonth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name     string
		sessions []domain.Session
		want     map[int]int
	}{
		{
			name:     "empty sessions",
			sessions: []domain.Session{},
			want:     map[int]int{},
		},
		{
			name: "sessions in different months",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 15, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 15, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 20, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 20, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(2).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.March, 10, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(2).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.March, 10, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(3).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.December, 25, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(3).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.December, 25, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(4).Build()).Build(),
				},
			},
			want: map[int]int{
				int(time.January):  2,
				int(time.March):    1,
				int(time.December): 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeSessionsPerMonth(ctx, tt.sessions)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestComputeFlawlessSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)
	start := time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		sessions []domain.Session
		want     *flawlessSessionStats
	}{
		{
			name:     "empty sessions",
			sessions: []domain.Session{},
			want:     nil,
		},
		{
			name: "no flawless sessions - has losses",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start.Add(2*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(3*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(1).Build()).Build(),
				},
			},
			want: &flawlessSessionStats{
				Count:      0,
				Percentage: 0,
			},
		},
		{
			name: "all flawless sessions",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(5).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start.Add(2*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(5).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(3*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(10).Build()).Build(),
				},
			},
			want: &flawlessSessionStats{
				Count:      2,
				Percentage: 100,
			},
		},
		{
			name: "mixed sessions - 2 flawless out of 4",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(2).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start.Add(2*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(2).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(3*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(2).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start.Add(4*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(2).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(5*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(5).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, start.Add(6*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(5).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, start.Add(7*time.Hour)).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(1).WithWins(5).Build()).Build(),
				},
			},
			want: &flawlessSessionStats{
				Count:      2,
				Percentage: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeFlawlessSessions(ctx, tt.sessions)
			if tt.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tt.want.Count, got.Count)
				require.InDelta(t, tt.want.Percentage, got.Percentage, 0.0001)
			}
		})
	}
}

func TestComputeAverages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name     string
		sessions []domain.Session
		want     *averageStats
	}{
		{
			name:     "empty sessions",
			sessions: []domain.Session{},
			want:     nil,
		},
		{
			name: "single session",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).WithWins(0).WithFinalKills(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(10).WithWins(5).WithFinalKills(20).Build()).Build(),
				},
			},
			want: &averageStats{
				SessionLength: 2.0,
				GamesPlayed:   10.0,
				Wins:          5.0,
				FinalKills:    20.0,
			},
		},
		{
			name: "multiple sessions",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).WithWins(0).WithFinalKills(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(10).WithWins(5).WithFinalKills(20).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 14, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(10).WithWins(5).WithFinalKills(20).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 18, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(30).WithWins(15).WithFinalKills(60).Build()).Build(),
				},
			},
			want: &averageStats{
				SessionLength: (2.0 + 4.0) / 2,
				GamesPlayed:   (10.0 + 20.0) / 2,
				Wins:          (5.0 + 10.0) / 2,
				FinalKills:    (20.0 + 40.0) / 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeAverages(ctx, tt.sessions)
			if tt.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.InDelta(t, tt.want.SessionLength, got.SessionLength, 0.0001)
				require.InDelta(t, tt.want.GamesPlayed, got.GamesPlayed, 0.0001)
				require.InDelta(t, tt.want.Wins, got.Wins, 0.0001)
				require.InDelta(t, tt.want.FinalKills, got.FinalKills, 0.0001)
			}
		})
	}
}

func TestComputeYearBoundaryStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name       string
		playerPITs []domain.PlayerPIT
		year       int
		wantStart  *time.Time
		wantEnd    *time.Time
	}{
		{
			name:       "empty PITs",
			playerPITs: []domain.PlayerPIT{},
			year:       2023,
			wantStart:  nil,
			wantEnd:    nil,
		},
		{
			name: "single PIT in year",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 15, 12, 0, 0, 0, time.UTC)).Build(),
			},
			year:      2023,
			wantStart: timePtr(time.Date(2023, time.June, 15, 12, 0, 0, 0, time.UTC)),
			wantEnd:   timePtr(time.Date(2023, time.June, 15, 12, 0, 0, 0, time.UTC)),
		},
		{
			name: "multiple PITs in year",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 15, 12, 0, 0, 0, time.UTC)).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.December, 31, 23, 59, 59, 0, time.UTC)).Build(),
			},
			year:      2023,
			wantStart: timePtr(time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)),
			wantEnd:   timePtr(time.Date(2023, time.December, 31, 23, 59, 59, 0, time.UTC)),
		},
		{
			name: "PITs outside year",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2022, time.December, 31, 23, 59, 59, 0, time.UTC)).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)).Build(),
			},
			year:      2023,
			wantStart: nil,
			wantEnd:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeYearBoundaryStats(ctx, tt.playerPITs, tt.year)
			if tt.wantStart == nil && tt.wantEnd == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				if tt.wantStart != nil {
					require.NotNil(t, got.Start)
					require.Equal(t, *tt.wantStart, got.Start.QueriedAt)
				}
				if tt.wantEnd != nil {
					require.NotNil(t, got.End)
					require.Equal(t, *tt.wantEnd, got.End.QueriedAt)
				}
			}
		})
	}
}

func TestComputeCoverage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name            string
		playerPITs      []domain.PlayerPIT
		sessions        []domain.Session
		year            int
		wantCoverage    float64
		wantAdjustedMin float64
		expectNil       bool
	}{
		{
			name:       "empty data",
			playerPITs: []domain.PlayerPIT{},
			sessions:   []domain.Session{},
			year:       2023,
			expectNil:  true,
		},
		{
			name: "100% coverage",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.December, 31, 23, 59, 59, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(100).Build()).Build(),
			},
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(100).Build()).Build(),
				},
			},
			year:            2023,
			wantCoverage:    100.0,
			wantAdjustedMin: 1.9,
		},
		{
			name: "50% coverage",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.December, 31, 23, 59, 59, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(100).Build()).Build(),
			},
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(50).Build()).Build(),
				},
			},
			year:            2023,
			wantCoverage:    50.0,
			wantAdjustedMin: 3.9,
		},
		{
			name: "coverage over 100%",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.December, 31, 23, 59, 59, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(50).Build()).Build(),
			},
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.June, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(100).Build()).Build(),
				},
			},
			year:            2023,
			wantCoverage:    200.0,
			wantAdjustedMin: 1.9, // Should keep sessionDuration when > 100%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeCoverage(ctx, tt.playerPITs, tt.sessions, tt.year)
			if tt.expectNil {
				if got != nil && len(tt.playerPITs) == 0 {
					require.Equal(t, 0.0, got.GamesPlayedPercentage)
					require.Equal(t, 0.0, got.AdjustedTotalHours)
				}
			} else {
				require.NotNil(t, got)
				require.InDelta(t, tt.wantCoverage, got.GamesPlayedPercentage, 0.01)
				require.GreaterOrEqual(t, got.AdjustedTotalHours, tt.wantAdjustedMin)
			}
		})
	}
}

func TestComputeBestSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name              string
		sessions          []domain.Session
		wantHighestFKDR   *float64
		wantMostKills     *int
		wantMostFinals    *int
		wantMostWins      *int
		wantLongestHours  *float64
		wantWinsPerHour   *float64
		wantFinalsPerHour *float64
	}{
		{
			name:     "empty sessions",
			sessions: []domain.Session{},
		},
		{
			name: "single session",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(0).WithFinalKills(0).WithWins(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(50).WithFinalKills(20).WithWins(5).WithFinalDeaths(2).Build()).Build(),
				},
			},
			wantHighestFKDR:   float64Ptr(10.0), // 20/2
			wantMostKills:     intPtr(50),
			wantMostFinals:    intPtr(20),
			wantMostWins:      intPtr(5),
			wantLongestHours:  float64Ptr(2.0),
			wantWinsPerHour:   float64Ptr(2.5),  // 5/2
			wantFinalsPerHour: float64Ptr(10.0), // 20/2
		},
		{
			name: "FKDR with zero final deaths - should use final kills as FKDR",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(0).WithFinalKills(0).WithWins(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(50).WithFinalKills(15).WithWins(5).WithFinalDeaths(0).Build()).Build(),
				},
			},
			wantHighestFKDR:   float64Ptr(15.0), // 15 final kills with 0 deaths = 15
			wantMostKills:     intPtr(50),
			wantMostFinals:    intPtr(15),
			wantMostWins:      intPtr(5),
			wantLongestHours:  float64Ptr(2.0),
			wantWinsPerHour:   float64Ptr(2.5),
			wantFinalsPerHour: float64Ptr(7.5),
		},
		{
			name: "multiple sessions with different bests",
			sessions: []domain.Session{
				// Session 1: 1 hour, FKDR=10 (10/1), 100 kills, 10 finals, 2 wins
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(0).WithFinalKills(0).WithWins(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(100).WithFinalKills(10).WithWins(2).WithFinalDeaths(1).Build()).Build(),
				},
				// Session 2: 8 hours (longest), FKDR=40 (40/1, highest), 50 kills, 40 finals (most), 20 wins (most)
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(100).WithFinalKills(10).WithWins(2).WithFinalDeaths(1).Build()).Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 18, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(150).WithFinalKills(50).WithWins(22).WithFinalDeaths(2).Build()).Build(),
				},
			},
			wantHighestFKDR:   float64Ptr(40.0), // Session 2: 40/1
			wantMostKills:     intPtr(100),      // Session 1: 100 kills
			wantMostFinals:    intPtr(40),       // Session 2: 40 finals
			wantMostWins:      intPtr(20),       // Session 2: 20 wins
			wantLongestHours:  float64Ptr(8.0),  // Session 2: 8 hours
			wantWinsPerHour:   float64Ptr(2.5),  // Session 2: 20/8 = 2.5
			wantFinalsPerHour: float64Ptr(10.0), // Session 1: 10/1 = 10.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeBestSessions(ctx, tt.sessions)
			if len(tt.sessions) == 0 {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				if tt.wantHighestFKDR != nil {
					require.NotNil(t, got.HighestFKDR)
					require.InDelta(t, *tt.wantHighestFKDR, got.HighestFKDR.Value, 0.01)
				}
				if tt.wantMostKills != nil {
					require.NotNil(t, got.MostKills)
					require.InDelta(t, *tt.wantMostKills, got.MostKills.Value, 0.01)
				}
				if tt.wantMostFinals != nil {
					require.NotNil(t, got.MostFinalKills)
					require.InDelta(t, *tt.wantMostFinals, got.MostFinalKills.Value, 0.01)
				}
				if tt.wantMostWins != nil {
					require.NotNil(t, got.MostWins)
					require.InDelta(t, *tt.wantMostWins, got.MostWins.Value, 0.01)
				}
				if tt.wantLongestHours != nil {
					require.NotNil(t, got.LongestSession)
					require.InDelta(t, *tt.wantLongestHours, got.LongestSession.Value, 0.01)
				}
				if tt.wantWinsPerHour != nil {
					require.NotNil(t, got.MostWinsPerHour)
					require.InDelta(t, *tt.wantWinsPerHour, got.MostWinsPerHour.Value, 0.01)
				}
				if tt.wantFinalsPerHour != nil {
					require.NotNil(t, got.MostFinalsPerHour)
					require.InDelta(t, *tt.wantFinalsPerHour, got.MostFinalsPerHour.Value, 0.01)
				}
			}
		})
	}
}

func TestComputeWinstreaks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name            string
		playerPITs      []domain.PlayerPIT
		wantOverallHigh int
	}{
		{
			name:            "empty PITs",
			playerPITs:      []domain.PlayerPIT{},
			wantOverallHigh: 0,
		},
		{
			name: "winstreak of 5 then loss",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(100).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(105).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(105).WithLosses(1).Build()).Build(),
			},
			wantOverallHigh: 5,
		},
		{
			name: "ongoing winstreak excluded",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(0).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(6).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(6).WithLosses(1).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.February, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(6).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.February, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(100).WithLosses(0).Build()).Build(),
			},
			wantOverallHigh: 6,
		},
		{
			name: "concurrent wins and losses - extra wins don't count",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(0).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(5).WithLosses(0).Build()).Build(),
				// Both wins and losses increased - we don't know the order
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(8).WithLosses(1).Build()).Build(),
			},
			wantOverallHigh: 5, // The extra 3 wins don't count toward the streak
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeWinstreaks(ctx, tt.playerPITs)

			if len(tt.playerPITs) == 0 {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.NotNil(t, got.Overall)

			require.Equal(t, tt.wantOverallHigh, got.Overall.Highest)
		})
	}
}

func TestComputeFinalKillStreaks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name            string
		playerPITs      []domain.PlayerPIT
		wantOverallHigh int
	}{
		{
			name:            "empty PITs",
			playerPITs:      []domain.PlayerPIT{},
			wantOverallHigh: 0,
		},
		{
			name: "final kill streak of 8 then death",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(100).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(108).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(108).WithFinalDeaths(1).Build()).Build(),
			},
			wantOverallHigh: 8,
		},
		{
			name: "ongoing streak excluded",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(0).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(9).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(9).WithFinalDeaths(1).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.February, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(9).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.February, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(100).WithFinalDeaths(0).Build()).Build(),
			},
			wantOverallHigh: 9,
		},
		{
			name: "concurrent final kills and deaths - extra kills don't count",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(0).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(8).WithFinalDeaths(0).Build()).Build(),
				// Both final kills and deaths increased - we don't know the order
				domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(12).WithFinalDeaths(1).Build()).Build(),
			},
			wantOverallHigh: 8, // The extra 4 kills don't count toward the streak
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeFinalKillStreaks(ctx, tt.playerPITs)

			if len(tt.playerPITs) == 0 {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.NotNil(t, got.Overall)

			require.Equal(t, tt.wantOverallHigh, got.Overall.Highest)
		})
	}
}

func TestComputeFavoritePlayIntervals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name           string
		sessions       []domain.Session
		wantMinResults int
		wantMaxResults int
	}{
		{
			name:           "empty sessions",
			sessions:       []domain.Session{},
			wantMinResults: 0,
			wantMaxResults: 0,
		},
		{
			name: "single 4-hour session",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 18, 0, 0, 0, time.UTC)).Build(),
				},
			},
			wantMinResults: 1,
			wantMaxResults: 3,
		},
		{
			name: "multiple sessions with clear afternoon favorite",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 18, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 18, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 3, 9, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 3, 10, 0, 0, 0, time.UTC)).Build(),
				},
			},
			wantMinResults: 1,
			wantMaxResults: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeFavoritePlayIntervals(ctx, tt.sessions)
			if tt.wantMaxResults == 0 {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.GreaterOrEqual(t, len(got), tt.wantMinResults)
				require.LessOrEqual(t, len(got), tt.wantMaxResults)
				// Check that intervals are sorted by percentage (descending)
				for i := 1; i < len(got); i++ {
					require.GreaterOrEqual(t, got[i-1].Percentage, got[i].Percentage)
				}
			}
		})
	}
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func TestComputePlaytimeDistribution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	playerUUID := domaintest.NewUUID(t)

	tests := []struct {
		name                    string
		sessions                []domain.Session
		wantHourlyDistribution  [24]float64
		wantDayHourDistribution map[string][24]float64
		wantNil                 bool
	}{
		{
			name:     "empty sessions",
			sessions: []domain.Session{},
			wantNil:  true,
		},
		{
			name: "single session within one hour",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 30, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).
						Build(),
				},
			},
			wantHourlyDistribution: func() [24]float64 {
				var arr [24]float64
				arr[10] = 0.5 // 30 minutes = 0.5 hours at hour 10
				return arr
			}(),
			wantDayHourDistribution: map[string][24]float64{
				"Sunday": {
					0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0,
					10: 0.5, // 30 minutes at hour 10
					11: 0, 12: 0, 13: 0, 14: 0, 15: 0, 16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0, 23: 0,
				},
			},
		},
		{
			name: "single session spanning two hours",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 14, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 16, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).
						Build(),
				},
			},
			wantHourlyDistribution: func() [24]float64 {
				var arr [24]float64
				arr[14] = 1.0 // hour 14: full hour
				arr[15] = 1.0 // hour 15: full hour
				return arr
			}(),
			wantDayHourDistribution: map[string][24]float64{
				"Monday": {
					0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0,
					10: 0, 11: 0, 12: 0, 13: 0,
					14: 1.0, // full hour
					15: 1.0, // full hour
					16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0, 23: 0,
				},
			},
		},
		{
			name: "session spanning partial hours",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 3, 9, 30, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 3, 11, 45, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).
						Build(),
				},
			},
			wantHourlyDistribution: func() [24]float64 {
				var arr [24]float64
				arr[9] = 0.5   // 9:30-10:00 = 30 min = 0.5 hours
				arr[10] = 1.0  // 10:00-11:00 = 1 hour
				arr[11] = 0.75 // 11:00-11:45 = 45 min = 0.75 hours
				return arr
			}(),
			wantDayHourDistribution: map[string][24]float64{
				"Tuesday": {
					0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0,
					9:  0.5,  // 30 minutes
					10: 1.0,  // full hour
					11: 0.75, // 45 minutes
					12: 0, 13: 0, 14: 0, 15: 0, 16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0, 23: 0,
				},
			},
		},
		{
			name: "multiple sessions on different days",
			sessions: []domain.Session{
				{
					// Sunday session
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).
						Build(),
				},
				{
					// Monday session at the same hour
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(2).Build()).
						Build(),
				},
			},
			wantHourlyDistribution: func() [24]float64 {
				var arr [24]float64
				arr[10] = 2.0 // 1 hour on Sunday + 1 hour on Monday
				arr[11] = 1.0 // 1 hour on Sunday
				return arr
			}(),
			wantDayHourDistribution: map[string][24]float64{
				"Sunday": {
					0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0,
					10: 1.0, // 1 hour
					11: 1.0, // 1 hour
					12: 0, 13: 0, 14: 0, 15: 0, 16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0, 23: 0,
				},
				"Monday": {
					0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0,
					10: 1.0, // 1 hour
					11: 0, 12: 0, 13: 0, 14: 0, 15: 0, 16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0, 23: 0,
				},
			},
		},
		{
			name: "session crossing midnight",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 1, 23, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).Build()).
						Build(),
					End: domaintest.NewPlayerBuilder(playerUUID, time.Date(2023, time.January, 2, 1, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(1).Build()).
						Build(),
				},
			},
			wantHourlyDistribution: func() [24]float64 {
				var arr [24]float64
				arr[23] = 1.0 // hour 23 (Sunday)
				arr[0] = 1.0  // hour 0 (Monday)
				return arr
			}(),
			wantDayHourDistribution: map[string][24]float64{
				"Sunday": {
					0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0,
					10: 0, 11: 0, 12: 0, 13: 0, 14: 0, 15: 0, 16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0,
					23: 1.0, // 1 hour
				},
				"Monday": {
					0: 1.0, // 1 hour
					1: 0, 2: 0, 3: 0, 4: 0, 5: 0, 6: 0, 7: 0, 8: 0, 9: 0,
					10: 0, 11: 0, 12: 0, 13: 0, 14: 0, 15: 0, 16: 0, 17: 0, 18: 0, 19: 0,
					20: 0, 21: 0, 22: 0, 23: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computePlaytimeDistribution(ctx, tt.sessions)
			if tt.wantNil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tt.wantHourlyDistribution, got.HourlyDistribution, "hourly distribution mismatch")
				require.Equal(t, tt.wantDayHourDistribution, got.DayHourDistribution, "day-hour distribution mismatch")
			}
		})
	}
}
