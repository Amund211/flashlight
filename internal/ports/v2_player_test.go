package ports

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/require"
)

func TestMakeGetV2PlayerHandler(t *testing.T) {
	t.Parallel()

	const UUID = "01234567-89ab-cdef-0123-456789abcdef"

	now := time.Now()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	sentryMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return next
	}
	allowedOrigins, _ := NewDomainSuffixes("example.com")

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		player := domaintest.NewPlayerBuilder(UUID, now).WithExperience(1000).BuildPtr()

		getV2PlayerHandler := MakeGetV2PlayerHandler(func(ctx context.Context, uuid string) (*domain.PlayerPIT, error) {
			return player, nil
		}, allowedOrigins, logger, sentryMiddleware)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/player/"+UUID, nil)
		req.SetPathValue("uuid", UUID)
		
		getV2PlayerHandler(w, req)

		resp := w.Result()

		require.Equal(t, 200, resp.StatusCode)
		body := w.Body.String()
		
		require.Contains(t, body, UUID)
		require.Contains(t, body, `1000`)
		require.Contains(t, body, `"success":true`)
		require.Contains(t, body, `"player":{`)

		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("client error: invalid uuid", func(t *testing.T) {
		t.Parallel()

		getV2PlayerHandler := MakeGetV2PlayerHandler(func(ctx context.Context, uuid string) (*domain.PlayerPIT, error) {
			t.Helper()
			t.Fatal("should not be called")
			return nil, nil
		}, allowedOrigins, logger, sentryMiddleware)
		
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/player/1234-1234-1234", nil)
		req.SetPathValue("uuid", "1234-1234-1234")

		getV2PlayerHandler(w, req)

		resp := w.Result()
		require.Equal(t, 400, resp.StatusCode)
		
		body := w.Body.String()
		require.Contains(t, body, `"success":false`)
		require.Contains(t, body, `"cause":"Invalid UUID"`)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("player not found", func(t *testing.T) {
		t.Parallel()

		getV2PlayerHandler := MakeGetV2PlayerHandler(func(ctx context.Context, uuid string) (*domain.PlayerPIT, error) {
			return nil, fmt.Errorf("%w: couldn't find him", domain.ErrPlayerNotFound)
		}, allowedOrigins, logger, sentryMiddleware)
		
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/player/"+UUID, nil)
		req.SetPathValue("uuid", UUID)

		getV2PlayerHandler(w, req)

		resp := w.Result()
		require.Equal(t, 404, resp.StatusCode)
		
		body := w.Body.String()
		require.Contains(t, body, `"success":true`)
		require.Contains(t, body, `"player":null`)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("provider temporarily unavailable", func(t *testing.T) {
		t.Parallel()

		getV2PlayerHandler := MakeGetV2PlayerHandler(func(ctx context.Context, uuid string) (*domain.PlayerPIT, error) {
			return nil, fmt.Errorf("%w: hypixel down", domain.ErrTemporarilyUnavailable)
		}, allowedOrigins, logger, sentryMiddleware)
		
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/player/"+UUID, nil)
		req.SetPathValue("uuid", UUID)

		getV2PlayerHandler(w, req)

		resp := w.Result()
		require.Equal(t, 503, resp.StatusCode)
		
		body := w.Body.String()
		require.Contains(t, body, `"success":false`)
		require.Contains(t, body, `"cause":"Service temporarily unavailable"`)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("internal server error", func(t *testing.T) {
		t.Parallel()

		getV2PlayerHandler := MakeGetV2PlayerHandler(func(ctx context.Context, uuid string) (*domain.PlayerPIT, error) {
			return nil, fmt.Errorf("some unknown error")
		}, allowedOrigins, logger, sentryMiddleware)
		
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/player/"+UUID, nil)
		req.SetPathValue("uuid", UUID)

		getV2PlayerHandler(w, req)

		resp := w.Result()
		require.Equal(t, 500, resp.StatusCode)
		
		body := w.Body.String()
		require.Contains(t, body, `"success":false`)
		require.Contains(t, body, `"cause":"Internal server error"`)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("player with all fields", func(t *testing.T) {
		t.Parallel()

		displayname := "TestPlayer"
		lastLogin := now.Add(-1 * time.Hour)
		lastLogout := now.Add(-30 * time.Minute)

		player := &domain.PlayerPIT{
			QueriedAt:           now,
			UUID:                UUID,
			Displayname:         &displayname,
			LastLogin:           &lastLogin,
			LastLogout:          &lastLogout,
			MissingBedwarsStats: false,
			Experience:          2500.5,
			Solo: domain.GamemodeStatsPIT{
				Winstreak:   func() *int { i := 10; return &i }(),
				GamesPlayed: 100,
				Wins:        80,
				Losses:      20,
				BedsBroken:  150,
				BedsLost:    30,
				FinalKills:  200,
				FinalDeaths: 25,
				Kills:       500,
				Deaths:      100,
			},
			Doubles: domain.GamemodeStatsPIT{
				Winstreak:   nil,
				GamesPlayed: 50,
				Wins:        40,
				Losses:      10,
				BedsBroken:  75,
				BedsLost:    15,
				FinalKills:  100,
				FinalDeaths: 12,
				Kills:       250,
				Deaths:      50,
			},
			// Other gamemodes with default values
			Threes:  domain.GamemodeStatsPIT{},
			Fours:   domain.GamemodeStatsPIT{},
			Overall: domain.GamemodeStatsPIT{},
		}

		getV2PlayerHandler := MakeGetV2PlayerHandler(func(ctx context.Context, uuid string) (*domain.PlayerPIT, error) {
			return player, nil
		}, allowedOrigins, logger, sentryMiddleware)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/player/"+UUID, nil)
		req.SetPathValue("uuid", UUID)

		getV2PlayerHandler(w, req)

		resp := w.Result()
		require.Equal(t, 200, resp.StatusCode)
		
		body := w.Body.String()
		require.Contains(t, body, `"success":true`)
		require.Contains(t, body, `"displayname":"TestPlayer"`)
		require.Contains(t, body, `"experience":2500.5`)
		require.Contains(t, body, `"missingBedwarsStats":false`)
		require.Contains(t, body, `"winstreak":10`)
		require.Contains(t, body, `"gamesPlayed":100`)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})
}