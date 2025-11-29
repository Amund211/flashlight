package ports_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/adapters/playerrepository"
	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/Amund211/flashlight/internal/ports"
	"github.com/stretchr/testify/require"
)

type mockPlayerRepository struct {
	playerrepository.StubPlayerRepository
	t             *testing.T
	expectedUUID  string
	expectedStart time.Time
	expectedEnd   time.Time
	stats         []domain.PlayerPIT
	err           error
	called        *bool
}

func (m *mockPlayerRepository) GetPlayerPITs(ctx context.Context, uuid string, start, end time.Time) ([]domain.PlayerPIT, error) {
	require.Equal(m.t, m.expectedUUID, uuid)
	// Account for the 24-hour padding
	require.WithinDuration(m.t, m.expectedStart.Add(-24*time.Hour), start, 0)
	require.WithinDuration(m.t, m.expectedEnd.Add(24*time.Hour), end, 0)

	*m.called = true

	return m.stats, m.err
}

func TestMakeGetSessionsHandler(t *testing.T) {
	t.Parallel()

	allowedOrigins, err := ports.NewDomainSuffixes("example.com", "test.com")
	require.NoError(t, err)

	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	noopMiddleware := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(w, r)
		}
	}

	makePlayerRepo := func(t *testing.T, expectedUUID string, expectedStart, expectedEnd time.Time, stats []domain.PlayerPIT, err error) (playerrepository.PlayerRepository, *bool) {
		called := false
		return &mockPlayerRepository{
			t:             t,
			expectedUUID:  expectedUUID,
			expectedStart: expectedStart,
			expectedEnd:   expectedEnd,
			stats:         stats,
			err:           err,
			called:        &called,
		}, &called
	}

	noopUpdatePlayerInInterval := func(ctx context.Context, uuid string, start, end time.Time) error {
		return nil
	}

	makeGetSessionsHandler := func(playerRepo playerrepository.PlayerRepository, updatePlayerInInterval app.UpdatePlayerInInterval) http.HandlerFunc {
		return ports.MakeGetSessionsHandler(
			playerRepo,
			updatePlayerInInterval,
			allowedOrigins,
			testLogger,
			noopMiddleware,
		)
	}

	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	startStr := "2023-01-01T00:00:00Z"
	end := time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC)
	endStr := "2023-01-01T01:00:00Z"
	
	// Create player PITs that will be returned by GetPlayerPITs
	stats := []domain.PlayerPIT{
		domaintest.NewPlayerBuilder(uuid, start).
			WithExperience(500).
			WithGamesPlayed(10).
			WithOverallStats(
				domaintest.NewStatsBuilder().WithGamesPlayed(10).WithFinalKills(10).Build(),
			).FromDB().Build(),
		domaintest.NewPlayerBuilder(uuid, end).
			WithExperience(1000).
			WithGamesPlayed(11).
			WithOverallStats(
				domaintest.NewStatsBuilder().WithGamesPlayed(11).WithFinalKills(11).Build(),
			).FromDB().Build(),
	}
	
	// Expected sessions computed from stats
	sessions := []domain.Session{
		{
			Start: stats[0],
			End:   stats[1],
			Consecutive: true,
		},
	}
	sessionsJSON, err := ports.SessionsToRainbowSessionsData(sessions)
	require.NoError(t, err)

	makeRequest := func(
		uuid string,
		startStr, endStr string,
	) *http.Request {
		body := io.NopCloser(
			strings.NewReader(
				fmt.Sprintf(
					`{"uuid":"%s","start":"%s","end":"%s"}`,
					uuid,
					startStr,
					endStr,
				),
			),
		)
		return httptest.NewRequest("GET", "/sessions", body)

	}

	t.Run("successful sessions retrieval", func(t *testing.T) {
		t.Parallel()

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest(uuid, startStr, endStr)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.NoError(t, err)
		require.JSONEq(t, string(sessionsJSON), w.Body.String())
		require.True(t, *called)
	})

	t.Run("start time == end time", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		startStr := "2023-01-01T00:00:00Z"
		end := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		endStr := "2023-01-01T00:00:00Z"

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest(uuid, startStr, endStr)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.NoError(t, err)
		require.JSONEq(t, string(sessionsJSON), w.Body.String())
		require.True(t, *called)
	})

	t.Run("invalid UUID format", func(t *testing.T) {
		t.Parallel()

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest("invalid-uuid", startStr, endStr)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid uuid")
		require.False(t, *called)
	})

	t.Run("start time after end time", func(t *testing.T) {
		t.Parallel()

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest(uuid, "2023-01-01T00:00:00.000000001Z", "2023-01-01T00:00:00.000000000Z")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Start time cannot be after end time")
		require.False(t, *called)
	})

	t.Run("interval exactly 400 days", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		startStr := "2023-01-01T00:00:00Z"
		end := start.Add(400 * 24 * time.Hour)
		endStr := end.Format(time.RFC3339)

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest(uuid, startStr, endStr)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Time interval is too long")
		require.False(t, *called)
	})

	t.Run("interval more than 400 days", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		startStr := "2023-01-01T00:00:00Z"
		end := start.Add(401 * 24 * time.Hour)
		endStr := end.Format(time.RFC3339)

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest(uuid, startStr, endStr)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Time interval is too long")
		require.False(t, *called)
	})

	t.Run("interval just under 400 days", func(t *testing.T) {
		t.Parallel()

		start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		startStr := "2023-01-01T00:00:00Z"
		end := start.Add(399*24*time.Hour + 23*time.Hour + 59*time.Minute + 59*time.Second)
		endStr := end.Format(time.RFC3339)

		playerRepo, called := makePlayerRepo(t, uuid, start, end, stats, nil)
		handler := makeGetSessionsHandler(playerRepo, noopUpdatePlayerInInterval)

		req := makeRequest(uuid, startStr, endStr)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, *called)
	})
}
