package ports_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/Amund211/flashlight/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestMakeGetWrappedHandler(t *testing.T) {
	t.Parallel()

	allowedOrigins, err := ports.NewDomainSuffixes("example.com", "test.com")
	require.NoError(t, err)

	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	noopMiddleware := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(w, r)
		}
	}

	makeGetSessions := func(t *testing.T, expectedUUID string, expectedStart, expectedEnd time.Time, sessions []domain.Session, err error) (app.GetSessions, *bool) {
		called := false
		return func(ctx context.Context, uuid string, start, end time.Time) ([]domain.Session, error) {
			t.Helper()
			require.Equal(t, expectedUUID, uuid)
			require.Equal(t, expectedStart, start)
			require.Equal(t, expectedEnd, end)

			called = true

			return sessions, err
		}, &called
	}

	makeGetWrappedHandler := func(getSessions app.GetSessions) http.HandlerFunc {
		return ports.MakeGetWrappedHandler(
			getSessions,
			allowedOrigins,
			testLogger,
			noopMiddleware,
		)
	}

	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	year := 2023
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)

	session1Start := time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC)
	session1End := time.Date(2023, 6, 1, 15, 0, 0, 0, time.UTC) // 5 hours
	session2Start := time.Date(2023, 7, 1, 10, 0, 0, 0, time.UTC)
	session2End := time.Date(2023, 7, 1, 12, 0, 0, 0, time.UTC) // 2 hours

	sessions := []domain.Session{
		{
			Start: domaintest.NewPlayerBuilder(uuid, session1Start).
				WithExperience(500).
				WithOverallStats(
					domain.GamemodeStatsPIT{
						FinalKills:  10,
						FinalDeaths: 5,
						Wins:        5,
						GamesPlayed: 5,
					},
				).Build(),
			End: domaintest.NewPlayerBuilder(uuid, session1End).
				WithExperience(1000).
				WithOverallStats(
					domain.GamemodeStatsPIT{
						FinalKills:  20,
						FinalDeaths: 10,
						Wins:        10,
						GamesPlayed: 10,
					},
				).Build(),
			Consecutive: true,
		},
		{
			Start: domaintest.NewPlayerBuilder(uuid, session2Start).
				WithExperience(1000).
				WithOverallStats(
					domain.GamemodeStatsPIT{
						FinalKills:  20,
						FinalDeaths: 10,
						Wins:        10,
						GamesPlayed: 10,
					},
				).Build(),
			End: domaintest.NewPlayerBuilder(uuid, session2End).
				WithExperience(1200).
				WithOverallStats(
					domain.GamemodeStatsPIT{
						FinalKills:  40,
						FinalDeaths: 11,
						Wins:        20,
						GamesPlayed: 20,
					},
				).Build(),
			Consecutive: false,
		},
	}

	makeRequest := func(uuid string, year string) *http.Request {
		req := httptest.NewRequest("GET", "/wrapped/"+uuid+"/"+year, nil)
		req.SetPathValue("uuid", uuid)
		req.SetPathValue("year", year)
		return req
	}

	t.Run("successful wrapped retrieval", func(t *testing.T) {
		t.Parallel()

		getSessionsFunc, called := makeGetSessions(t, uuid, start, end, sessions, nil)
		handler := makeGetWrappedHandler(getSessionsFunc)

		req := makeRequest(uuid, "2023")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, *called)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		require.Equal(t, true, response["success"])
		require.Equal(t, uuid, response["uuid"])
		require.Equal(t, float64(year), response["year"])
		require.Equal(t, float64(2), response["totalSessions"])

		// Verify longest session (session1 is 5 hours)
		longestSession := response["longestSession"].(map[string]interface{})
		require.NotNil(t, longestSession)
		require.Equal(t, 5.0, longestSession["durationHours"])

		// Verify highest FKDR (session2 has 20/1 = 20 FKDR)
		highestFKDR := response["highestFKDR"].(map[string]interface{})
		require.NotNil(t, highestFKDR)
		require.Equal(t, 20.0, highestFKDR["fkdr"])

		// Verify total stats
		totalStats := response["totalStats"].(map[string]interface{})
		require.NotNil(t, totalStats)
		require.Equal(t, float64(30), totalStats["finalKills"])  // 10 + 20
		require.Equal(t, float64(6), totalStats["finalDeaths"])  // 5 + 1
		require.Equal(t, float64(15), totalStats["wins"])        // 5 + 10
	})

	t.Run("empty sessions", func(t *testing.T) {
		t.Parallel()

		getSessionsFunc, called := makeGetSessions(t, uuid, start, end, []domain.Session{}, nil)
		handler := makeGetWrappedHandler(getSessionsFunc)

		req := makeRequest(uuid, "2023")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, *called)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		require.Equal(t, true, response["success"])
		require.Equal(t, float64(0), response["totalSessions"])
		require.Nil(t, response["longestSession"])
		require.Nil(t, response["highestFKDR"])
	})

	t.Run("invalid UUID format", func(t *testing.T) {
		t.Parallel()

		getSessionsFunc, called := makeGetSessions(t, uuid, start, end, sessions, nil)
		handler := makeGetWrappedHandler(getSessionsFunc)

		req := makeRequest("invalid-uuid", "2023")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, *called)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		require.Equal(t, false, response["success"])
		require.Equal(t, "Invalid UUID", response["cause"])
	})

	t.Run("invalid year format", func(t *testing.T) {
		t.Parallel()

		getSessionsFunc, called := makeGetSessions(t, uuid, start, end, sessions, nil)
		handler := makeGetWrappedHandler(getSessionsFunc)

		req := makeRequest(uuid, "not-a-year")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, *called)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		require.Equal(t, false, response["success"])
		require.Equal(t, "Invalid year", response["cause"])
	})

	t.Run("year out of range", func(t *testing.T) {
		t.Parallel()

		getSessionsFunc, called := makeGetSessions(t, uuid, start, end, sessions, nil)
		handler := makeGetWrappedHandler(getSessionsFunc)

		req := makeRequest(uuid, "1999")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, *called)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		require.Equal(t, false, response["success"])
		require.Equal(t, "Invalid year", response["cause"])
	})
}
