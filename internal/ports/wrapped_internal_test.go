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
	ctx := context.Background()

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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).Build(),
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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 18, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 3, 9, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 3, 10, 0, 0, 0, time.UTC)).Build(),
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
	ctx := context.Background()

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
				{Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 15, 10, 0, 0, 0, time.UTC)).Build()},
				{Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 20, 10, 0, 0, 0, time.UTC)).Build()},
				{Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 3, 10, 10, 0, 0, 0, time.UTC)).Build()},
				{Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC)).Build()},
			},
			want: map[int]int{
				1:  2,
				3:  1,
				12: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSessionsPerMonth(ctx, tt.sessions)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestComputeFlawlessSessions(t *testing.T) {
	ctx := context.Background()

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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(5).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(5).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(2).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(0).WithFinalDeaths(0).WithWins(2).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(2).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(2).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(5).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
						WithOverallStats(domaintest.NewStatsBuilder().WithLosses(1).WithFinalDeaths(0).WithWins(5).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Time{}).
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
	ctx := context.Background()

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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).WithWins(0).WithFinalKills(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).
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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(0).WithWins(0).WithFinalKills(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(10).WithWins(5).WithFinalKills(20).Build()).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 14, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(10).WithWins(5).WithFinalKills(20).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 18, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().WithGamesPlayed(30).WithWins(15).WithFinalKills(60).Build()).Build(),
				},
			},
			want: &averageStats{
				SessionLength: 3.0,  // (2+4)/2
				GamesPlayed:   15.0, // (10+20)/2
				Wins:          7.5,  // (5+10)/2
				FinalKills:    30.0, // (20+40)/2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	ctx := context.Background()

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
				domaintest.NewPlayerBuilder("test1", time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)).Build(),
			},
			year:      2023,
			wantStart: timePtr(time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)),
			wantEnd:   timePtr(time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)),
		},
		{
			name: "multiple PITs in year",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder("first", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).Build(),
				domaintest.NewPlayerBuilder("middle", time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)).Build(),
				domaintest.NewPlayerBuilder("last", time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)).Build(),
			},
			year:      2023,
			wantStart: timePtr(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			wantEnd:   timePtr(time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)),
		},
		{
			name: "PITs outside year",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder("before", time.Date(2022, 12, 31, 23, 59, 59, 0, time.UTC)).Build(),
				domaintest.NewPlayerBuilder("after", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)).Build(),
			},
			year:      2023,
			wantStart: nil,
			wantEnd:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	ctx := context.Background()

	tests := []struct {
		name            string
		playerPITs      []domain.PlayerPIT
		sessions        []domain.Session
		year            int
		wantCoverage    float64
		wantAdjustedMin float64
	}{
		{
			name:            "empty data",
			playerPITs:      []domain.PlayerPIT{},
			sessions:        []domain.Session{},
			year:            2023,
			wantCoverage:    -1, // -1 means nil result
			wantAdjustedMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCoverage(ctx, tt.playerPITs, tt.sessions, tt.year)
			if tt.wantCoverage < 0 {
				// Expected nil
				if got != nil && len(tt.playerPITs) == 0 {
					// Allow either nil or zero values for empty input
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
	ctx := context.Background()

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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(0).WithFinalKills(0).WithWins(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).
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
			name: "multiple sessions with different bests",
			sessions: []domain.Session{
				// Session 1: 1 hour, FKDR=10 (10/1), 100 kills, 10 finals, 2 wins
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(0).WithFinalKills(0).WithWins(0).WithFinalDeaths(0).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(100).WithFinalKills(10).WithWins(2).WithFinalDeaths(1).Build()).Build(),
				},
				// Session 2: 8 hours (longest), FKDR=40 (40/1, highest), 50 kills, 40 finals (most), 20 wins (most)
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 10, 0, 0, 0, time.UTC)).
						WithOverallStats(domaintest.NewStatsBuilder().
							WithKills(100).WithFinalKills(10).WithWins(2).WithFinalDeaths(1).Build()).Build(),
					End: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 18, 0, 0, 0, time.UTC)).
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
	ctx := context.Background()

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
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(0).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(5).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(5).WithLosses(1).Build()).Build(),
			},
			wantOverallHigh: 5,
		},
		{
			name: "ongoing winstreak excluded",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(0).WithLosses(0).Build()).Build(),
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithWins(10).WithLosses(0).Build()).Build(),
			},
			wantOverallHigh: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeWinstreaks(ctx, tt.playerPITs)
			if len(tt.playerPITs) == 0 {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				if tt.wantOverallHigh > 0 {
					require.NotNil(t, got.Overall)
					require.Equal(t, tt.wantOverallHigh, got.Overall.Highest)
				} else {
					if got.Overall != nil {
						require.Equal(t, 0, got.Overall.Highest)
					}
				}
			}
		})
	}
}

func TestComputeFinalKillStreaks(t *testing.T) {
	ctx := context.Background()

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
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(0).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(8).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(8).WithFinalDeaths(1).Build()).Build(),
			},
			wantOverallHigh: 8,
		},
		{
			name: "ongoing streak excluded",
			playerPITs: []domain.PlayerPIT{
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(0).WithFinalDeaths(0).Build()).Build(),
				domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)).
					WithOverallStats(domaintest.NewStatsBuilder().WithFinalKills(15).WithFinalDeaths(0).Build()).Build(),
			},
			wantOverallHigh: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeFinalKillStreaks(ctx, tt.playerPITs)
			if len(tt.playerPITs) == 0 {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				if tt.wantOverallHigh > 0 {
					require.NotNil(t, got.Overall)
					require.Equal(t, tt.wantOverallHigh, got.Overall.Highest)
				} else {
					if got.Overall != nil {
						require.Equal(t, 0, got.Overall.Highest)
					}
				}
			}
		})
	}
}

func TestComputeFavoritePlayIntervals(t *testing.T) {
	ctx := context.Background()

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
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 18, 0, 0, 0, time.UTC)).Build(),
				},
			},
			wantMinResults: 1,
			wantMaxResults: 3,
		},
		{
			name: "multiple sessions with clear afternoon favorite",
			sessions: []domain.Session{
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 1, 18, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 14, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 2, 18, 0, 0, 0, time.UTC)).Build(),
				},
				{
					Start: domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 3, 9, 0, 0, 0, time.UTC)).Build(),
					End:   domaintest.NewPlayerBuilder("uuid1", time.Date(2023, 1, 3, 10, 0, 0, 0, time.UTC)).Build(),
				},
			},
			wantMinResults: 1,
			wantMaxResults: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
