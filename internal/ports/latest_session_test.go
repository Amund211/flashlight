package ports_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/Amund211/flashlight/internal/ports"
)

func TestMakeGetLatestSessionHandler(t *testing.T) {
	t.Parallel()

	allowedOrigins, err := ports.NewDomainSuffixes("example.com", "test.com")
	require.NoError(t, err)

	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	noopMiddleware := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(w, r)
		}
	}

	makeHandler := func(getLatestSession app.GetLatestSession) http.HandlerFunc {
		stubRegisterUserVisit := func(ctx context.Context, userID string, ipHash string, userAgent string) (domain.User, error) {
			return domain.User{}, nil
		}
		return ports.MakeGetLatestSessionHandler(
			getLatestSession,
			stubRegisterUserVisit,
			allowedOrigins,
			testLogger,
			noopMiddleware,
			emptyBlocklistConfig,
		)
	}

	makeRequest := func(uuid string) *http.Request {
		body := io.NopCloser(strings.NewReader(
			fmt.Sprintf(`{"uuid":"%s"}`, uuid),
		))
		return httptest.NewRequestWithContext(t.Context(), "POST", "/session-at/latest", body)
	}

	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	at := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	type gameResponse struct {
		Gamemode   string `json:"gamemode"`
		Outcome    string `json:"outcome"`
		FinalKills int    `json:"finalKills"`
		FinalDeath bool   `json:"finalDeath"`
		BedsBroken int    `json:"bedsBroken"`
		BedLost    bool   `json:"bedLost"`
		Kills      int    `json:"kills"`
		Deaths     int    `json:"deaths"`
		Experience int64  `json:"experience"`
	}
	type segmentResponse struct {
		Start map[string]any `json:"start"`
		End   map[string]any `json:"end"`
		Game  *gameResponse  `json:"game"`
	}
	type sessionAtResponse struct {
		Session *struct {
			Start       map[string]any `json:"start"`
			End         map[string]any `json:"end"`
			Consecutive bool           `json:"consecutive"`
		} `json:"session"`
		Games []segmentResponse `json:"games"`
	}

	t.Run("forwards uuid to app method and renders games", func(t *testing.T) {
		t.Parallel()

		sessionStart := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
		mid := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		sessionEnd := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)

		startPIT := domaintest.NewPlayerBuilder(uuid).
			WithExperience(1000).FromDB().Fours().WithGamesPlayed(10).Build(sessionStart)
		midPIT := domaintest.NewPlayerBuilder(uuid).
			WithExperience(1500).FromDB().Fours().WithGamesPlayed(11).Build(mid)
		endPIT := domaintest.NewPlayerBuilder(uuid).
			WithExperience(2000).FromDB().Fours().WithGamesPlayed(12).Build(sessionEnd)

		result := app.SessionAtResult{
			Session: &domain.Session{
				Start:       startPIT,
				End:         endPIT,
				Consecutive: true,
			},
			Games: []app.GameSegment{
				{
					Start: startPIT,
					End:   midPIT,
					Game: &domain.GameResult{
						Gamemode:   domain.GamemodeDoubles,
						Outcome:    domain.GameOutcomeWin,
						FinalKills: 4,
						FinalDeath: false,
						BedsBroken: 1,
						BedLost:    false,
						Kills:      8,
						Deaths:     2,
						Experience: 500,
					},
				},
				{Start: midPIT, End: endPIT, Game: nil},
			},
		}

		called := false
		getLatestSession := func(ctx context.Context, gotUUID string) (app.SessionAtResult, error) {
			called = true
			require.Equal(t, uuid, gotUUID)
			return result, nil
		}

		handler := makeHandler(getLatestSession)
		req := makeRequest(uuid)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, called)

		var response sessionAtResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

		require.NotNil(t, response.Session)
		require.True(t, response.Session.Consecutive)
		require.Equal(t, sessionStart.Format(time.RFC3339), response.Session.Start["queriedAt"])
		require.Equal(t, sessionEnd.Format(time.RFC3339), response.Session.End["queriedAt"])

		require.Len(t, response.Games, 2)
		require.NotNil(t, response.Games[0].Game)
		require.Equal(t, "doubles", response.Games[0].Game.Gamemode)
		require.Equal(t, "win", response.Games[0].Game.Outcome)
		require.Equal(t, int64(500), response.Games[0].Game.Experience)
		require.Nil(t, response.Games[1].Game)
	})

	t.Run("nil session is rendered as null with empty games", func(t *testing.T) {
		t.Parallel()

		getLatestSession := func(ctx context.Context, gotUUID string) (app.SessionAtResult, error) {
			return app.SessionAtResult{Session: nil, Games: nil}, nil
		}

		handler := makeHandler(getLatestSession)
		req := makeRequest(uuid)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response sessionAtResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Nil(t, response.Session)
		require.Empty(t, response.Games)
	})

	t.Run("unknown gamemode returns 500", func(t *testing.T) {
		t.Parallel()

		startPIT := domaintest.NewPlayerBuilder(uuid).FromDB().Build(at)

		result := app.SessionAtResult{
			Session: &domain.Session{Start: startPIT, End: startPIT, Consecutive: true},
			Games: []app.GameSegment{
				{
					Start: startPIT,
					End:   startPIT,
					Game:  &domain.GameResult{Gamemode: domain.Gamemode("bogus")},
				},
			},
		}

		getLatestSession := func(ctx context.Context, _ string) (app.SessionAtResult, error) {
			return result, nil
		}

		handler := makeHandler(getLatestSession)
		req := makeRequest(uuid)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	makeAssertNotCalled := func(t *testing.T) app.GetLatestSession {
		return func(ctx context.Context, uuid string) (app.SessionAtResult, error) {
			t.Helper()
			t.Fatal("getLatestSession should not be called")
			return app.SessionAtResult{}, nil
		}
	}

	t.Run("invalid UUID", func(t *testing.T) {
		t.Parallel()

		handler := makeHandler(makeAssertNotCalled(t))
		req := makeRequest("not-a-uuid")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid uuid")
	})

	t.Run("request body exceeds size limit", func(t *testing.T) {
		t.Parallel()

		handler := makeHandler(makeAssertNotCalled(t))
		oversized := fmt.Sprintf(`{"uuid":"%s"}`, strings.Repeat("a", 5<<10))
		body := io.NopCloser(strings.NewReader(oversized))
		req := httptest.NewRequestWithContext(t.Context(), "POST", "/session-at/latest", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()

		handler := makeHandler(makeAssertNotCalled(t))
		body := io.NopCloser(strings.NewReader("not json"))
		req := httptest.NewRequestWithContext(t.Context(), "POST", "/session-at/latest", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("app method failure returns 500", func(t *testing.T) {
		t.Parallel()

		getLatestSession := func(ctx context.Context, uuid string) (app.SessionAtResult, error) {
			return app.SessionAtResult{}, fmt.Errorf("boom")
		}

		handler := makeHandler(getLatestSession)
		req := makeRequest(uuid)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
