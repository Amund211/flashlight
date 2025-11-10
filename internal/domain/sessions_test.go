package domain_test

import (
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/require"
)

func TestSessionPlaytime(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	tests := []struct {
		name     string
		session  domain.Session
		expected time.Duration
	}{
		{
			name: "1 hour session",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).Build(),
				End:   domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).Build(),
			},
			expected: 1 * time.Hour,
		},
		{
			name: "5 hour session",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).Build(),
				End:   domaintest.NewPlayerBuilder(uuid, now.Add(5*time.Hour)).Build(),
			},
			expected: 5 * time.Hour,
		},
		{
			name: "30 minute session",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).Build(),
				End:   domaintest.NewPlayerBuilder(uuid, now.Add(30*time.Minute)).Build(),
			},
			expected: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.session.Playtime()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionFinalKills(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	tests := []struct {
		name     string
		session  domain.Session
		expected int
	}{
		{
			name: "10 final kills gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(110).
						Build()).
					Build(),
			},
			expected: 10,
		},
		{
			name: "0 final kills gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						Build()).
					Build(),
			},
			expected: 0,
		},
		{
			name: "50 final kills gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(200).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(250).
						Build()).
					Build(),
			},
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.session.FinalKills()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionWins(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	tests := []struct {
		name     string
		session  domain.Session
		expected int
	}{
		{
			name: "5 wins gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithWins(10).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithWins(15).
						Build()).
					Build(),
			},
			expected: 5,
		},
		{
			name: "0 wins gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithWins(20).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithWins(20).
						Build()).
					Build(),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.session.Wins()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionFKDR(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	tests := []struct {
		name     string
		session  domain.Session
		expected float64
	}{
		{
			name: "normal FKDR",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						WithFinalDeaths(50).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(120).
						WithFinalDeaths(60).
						Build()).
					Build(),
			},
			expected: 2.0, // 20 kills / 10 deaths
		},
		{
			name: "no deaths (infinite FKDR treated as kill count)",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						WithFinalDeaths(50).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(115).
						WithFinalDeaths(50).
						Build()).
					Build(),
			},
			expected: 15.0, // 15 kills / 0 deaths = 15
		},
		{
			name: "no kills and no deaths",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						WithFinalDeaths(50).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						WithFinalDeaths(50).
						Build()).
					Build(),
			},
			expected: 0.0,
		},
		{
			name: "high FKDR",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(100).
						WithFinalDeaths(50).
						Build()).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithOverallStats(domaintest.NewStatsBuilder().
						WithFinalKills(120).
						WithFinalDeaths(51).
						Build()).
					Build(),
			},
			expected: 20.0, // 20 kills / 1 death
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.session.FKDR()
			require.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestSessionStars(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	tests := []struct {
		name     string
		session  domain.Session
		expected float64
	}{
		{
			name: "stars gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithExperience(500).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithExperience(10000).
					Build(),
			},
			expected: domain.ExperienceToStars(10000) - domain.ExperienceToStars(500),
		},
		{
			name: "no stars gained",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithExperience(1000).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithExperience(1000).
					Build(),
			},
			expected: 0.0,
		},
		{
			name: "large star gain",
			session: domain.Session{
				Start: domaintest.NewPlayerBuilder(uuid, now).
					WithExperience(500).
					Build(),
				End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
					WithExperience(50000).
					Build(),
			},
			expected: domain.ExperienceToStars(50000) - domain.ExperienceToStars(500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.session.Stars()
			require.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestGetBest(t *testing.T) {
	t.Parallel()

	uuid := "12345678-1234-1234-1234-123456789012"
	now := time.Now()

	session1 := domain.Session{
		Start: domaintest.NewPlayerBuilder(uuid, now).
			WithExperience(500).
			WithOverallStats(domaintest.NewStatsBuilder().
				WithFinalKills(100).
				WithWins(10).
				Build()).
			Build(),
		End: domaintest.NewPlayerBuilder(uuid, now.Add(1*time.Hour)).
			WithExperience(1000).
			WithOverallStats(domaintest.NewStatsBuilder().
				WithFinalKills(110).
				WithWins(12).
				Build()).
			Build(),
	}

	session2 := domain.Session{
		Start: domaintest.NewPlayerBuilder(uuid, now.Add(2*time.Hour)).
			WithExperience(1000).
			WithOverallStats(domaintest.NewStatsBuilder().
				WithFinalKills(110).
				WithWins(12).
				Build()).
			Build(),
		End: domaintest.NewPlayerBuilder(uuid, now.Add(5*time.Hour)).
			WithExperience(2000).
			WithOverallStats(domaintest.NewStatsBuilder().
				WithFinalKills(130).
				WithWins(15).
				Build()).
			Build(),
	}

	t.Run("best playtime", func(t *testing.T) {
		t.Parallel()
		result := domain.GetBest(session1, session2, domain.Session.Playtime)
		require.Equal(t, session2, result) // session2 has 3 hours vs 1 hour
	})

	t.Run("best final kills", func(t *testing.T) {
		t.Parallel()
		result := domain.GetBest(session1, session2, domain.Session.FinalKills)
		require.Equal(t, session2, result) // session2 has 20 vs 10
	})

	t.Run("best wins", func(t *testing.T) {
		t.Parallel()
		result := domain.GetBest(session1, session2, domain.Session.Wins)
		require.Equal(t, session2, result) // session2 has 3 vs 2
	})

	t.Run("same value returns current", func(t *testing.T) {
		t.Parallel()
		result := domain.GetBest(session1, session1, domain.Session.FinalKills)
		require.Equal(t, session1, result)
	})
}
