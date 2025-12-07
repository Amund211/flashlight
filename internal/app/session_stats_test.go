package app_test

import (
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/require"
)

func TestComputeTotalSessions(t *testing.T) {
	t.Parallel()

	sessions := []domain.Session{
		{Consecutive: true},
		{Consecutive: false},
		{Consecutive: true},
	}

	result := app.ComputeTotalSessions(sessions)
	require.Equal(t, 3, result)
}

func TestComputeTotalConsecutiveSessions(t *testing.T) {
	t.Parallel()

	t.Run("all consecutive", func(t *testing.T) {
		t.Parallel()
		sessions := []domain.Session{
			{Consecutive: true},
			{Consecutive: true},
			{Consecutive: true},
		}

		result := app.ComputeTotalConsecutiveSessions(sessions)
		require.Equal(t, 3, result)
	})

	t.Run("mixed", func(t *testing.T) {
		t.Parallel()
		sessions := []domain.Session{
			{Consecutive: true},
			{Consecutive: false},
			{Consecutive: true},
			{Consecutive: false},
		}

		result := app.ComputeTotalConsecutiveSessions(sessions)
		require.Equal(t, 2, result)
	})

	t.Run("none consecutive", func(t *testing.T) {
		t.Parallel()
		sessions := []domain.Session{
			{Consecutive: false},
			{Consecutive: false},
		}

		result := app.ComputeTotalConsecutiveSessions(sessions)
		require.Equal(t, 0, result)
	})
}

func TestComputeStatsAtYearStart(t *testing.T) {
	t.Parallel()

	uuid := domaintest.NewUUID(t)

	stats := []domain.PlayerPIT{
		domaintest.NewPlayerBuilder(uuid, time.Date(2024, time.March, 15, 10, 0, 0, 0, time.UTC)).
			WithExperience(1000).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, time.Date(2024, time.January, 5, 8, 0, 0, 0, time.UTC)).
			WithExperience(500).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, time.Date(2024, time.December, 31, 23, 59, 0, 0, time.UTC)).
			WithExperience(2000).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, time.Date(2023, time.December, 31, 23, 59, 0, 0, time.UTC)).
			WithExperience(100).FromDB().Build(),
	}

	result := app.ComputeStatsAtYearStart(stats, 2024)
	require.NotNil(t, result)
	require.Equal(t, int64(500), result.Experience)
	require.Equal(t, 2024, result.QueriedAt.Year())
	require.Equal(t, time.January, result.QueriedAt.Month())
}

func TestComputeStatsAtYearEnd(t *testing.T) {
	t.Parallel()

	uuid := domaintest.NewUUID(t)

	stats := []domain.PlayerPIT{
		domaintest.NewPlayerBuilder(uuid, time.Date(2024, time.March, 15, 10, 0, 0, 0, time.UTC)).
			WithExperience(1000).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, time.Date(2024, time.January, 5, 8, 0, 0, 0, time.UTC)).
			WithExperience(500).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, time.Date(2024, time.December, 31, 23, 59, 0, 0, time.UTC)).
			WithExperience(2000).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, time.Date(2023, time.December, 31, 23, 59, 0, 0, time.UTC)).
			WithExperience(100).FromDB().Build(),
	}

	result := app.ComputeStatsAtYearEnd(stats, 2024)
	require.NotNil(t, result)
	require.Equal(t, int64(2000), result.Experience)
	require.Equal(t, 2024, result.QueriedAt.Year())
	require.Equal(t, time.December, result.QueriedAt.Month())
}

func TestComputeUTCTimeHistogram(t *testing.T) {
	t.Parallel()

	uuid := domaintest.NewUUID(t)

	// Create sessions at different hours
	sessions := []domain.Session{
		{
			Start: domaintest.NewPlayerBuilder(uuid, time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)).FromDB().Build(),
		},
		{
			Start: domaintest.NewPlayerBuilder(uuid, time.Date(2024, 1, 1, 0, 45, 0, 0, time.UTC)).FromDB().Build(),
		},
		{
			Start: domaintest.NewPlayerBuilder(uuid, time.Date(2024, 1, 1, 14, 15, 0, 0, time.UTC)).FromDB().Build(),
		},
		{
			Start: domaintest.NewPlayerBuilder(uuid, time.Date(2024, 1, 1, 14, 45, 0, 0, time.UTC)).FromDB().Build(),
		},
		{
			Start: domaintest.NewPlayerBuilder(uuid, time.Date(2024, 1, 1, 23, 59, 0, 0, time.UTC)).FromDB().Build(),
		},
	}

	result := app.ComputeUTCTimeHistogram(sessions)

	// Verify specific hours
	require.Equal(t, 2, result[0])  // midnight hour (00:00-01:00)
	require.Equal(t, 2, result[14]) // 2pm hour (14:00-15:00)
	require.Equal(t, 1, result[23]) // 11pm hour (23:00-00:00)

	// Verify other hours are zero
	for i := 1; i < 14; i++ {
		require.Equal(t, 0, result[i], "Hour %d should be 0", i)
	}
	for i := 15; i < 23; i++ {
		require.Equal(t, 0, result[i], "Hour %d should be 0", i)
	}

	// Verify total count
	total := 0
	for _, count := range result {
		total += count
	}
	require.Equal(t, len(sessions), total)
}
