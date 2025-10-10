package ports

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/require"
)

func TestDomainGamemodeStatsToV2(t *testing.T) {
	t.Parallel()

	t.Run("with winstreak", func(t *testing.T) {
		t.Parallel()

		winstreak := 15
		domainStats := &domain.GamemodeStatsPIT{
			Winstreak:   &winstreak,
			GamesPlayed: 100,
			Wins:        80,
			Losses:      20,
			BedsBroken:  150,
			BedsLost:    30,
			FinalKills:  200,
			FinalDeaths: 25,
			Kills:       500,
			Deaths:      100,
		}

		v2Stats := domainGamemodeStatsToV2(domainStats)

		require.Equal(t, &winstreak, v2Stats.Winstreak)
		require.Equal(t, 100, v2Stats.GamesPlayed)
		require.Equal(t, 80, v2Stats.Wins)
		require.Equal(t, 20, v2Stats.Losses)
		require.Equal(t, 150, v2Stats.BedsBroken)
		require.Equal(t, 30, v2Stats.BedsLost)
		require.Equal(t, 200, v2Stats.FinalKills)
		require.Equal(t, 25, v2Stats.FinalDeaths)
		require.Equal(t, 500, v2Stats.Kills)
		require.Equal(t, 100, v2Stats.Deaths)
	})

	t.Run("without winstreak", func(t *testing.T) {
		t.Parallel()

		domainStats := &domain.GamemodeStatsPIT{
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
		}

		v2Stats := domainGamemodeStatsToV2(domainStats)

		require.Nil(t, v2Stats.Winstreak)
		require.Equal(t, 50, v2Stats.GamesPlayed)
		require.Equal(t, 40, v2Stats.Wins)
		require.Equal(t, 10, v2Stats.Losses)
		require.Equal(t, 75, v2Stats.BedsBroken)
		require.Equal(t, 15, v2Stats.BedsLost)
		require.Equal(t, 100, v2Stats.FinalKills)
		require.Equal(t, 12, v2Stats.FinalDeaths)
		require.Equal(t, 250, v2Stats.Kills)
		require.Equal(t, 50, v2Stats.Deaths)
	})
}

func TestDomainPlayerToV2Player(t *testing.T) {
	t.Parallel()

	t.Run("nil player", func(t *testing.T) {
		t.Parallel()

		result := domainPlayerToV2Player(nil)
		require.Nil(t, result)
	})

	t.Run("full player", func(t *testing.T) {
		t.Parallel()

		const UUID = "01234567-89ab-cdef-0123-456789abcdef"
		now := time.Now()
		displayname := "TestPlayer"
		lastLogin := now.Add(-1 * time.Hour)
		lastLogout := now.Add(-30 * time.Minute)

		domainPlayer := &domain.PlayerPIT{
			QueriedAt:           now,
			UUID:                UUID,
			Displayname:         &displayname,
			LastLogin:           &lastLogin,
			LastLogout:          &lastLogout,
			MissingBedwarsStats: true,
			Experience:          1500.75,
			Solo: domain.GamemodeStatsPIT{
				Winstreak:   func() *int { i := 5; return &i }(),
				GamesPlayed: 50,
				Wins:        40,
			},
			Doubles: domain.GamemodeStatsPIT{
				GamesPlayed: 25,
				Wins:        20,
			},
			Threes: domain.GamemodeStatsPIT{
				GamesPlayed: 10,
				Wins:        8,
			},
			Fours: domain.GamemodeStatsPIT{
				GamesPlayed: 5,
				Wins:        4,
			},
			Overall: domain.GamemodeStatsPIT{
				GamesPlayed: 90,
				Wins:        72,
			},
		}

		v2Player := domainPlayerToV2Player(domainPlayer)

		require.NotNil(t, v2Player)
		require.Equal(t, now, v2Player.QueriedAt)
		require.Equal(t, UUID, v2Player.UUID)
		require.Equal(t, &displayname, v2Player.Displayname)
		require.Equal(t, &lastLogin, v2Player.LastLogin)
		require.Equal(t, &lastLogout, v2Player.LastLogout)
		require.Equal(t, true, v2Player.MissingBedwarsStats)
		require.Equal(t, 1500.75, v2Player.Experience)

		require.Equal(t, 50, v2Player.Solo.GamesPlayed)
		require.Equal(t, 40, v2Player.Solo.Wins)
		require.NotNil(t, v2Player.Solo.Winstreak)
		require.Equal(t, 5, *v2Player.Solo.Winstreak)

		require.Equal(t, 25, v2Player.Doubles.GamesPlayed)
		require.Equal(t, 20, v2Player.Doubles.Wins)
		require.Nil(t, v2Player.Doubles.Winstreak)
	})
}

func TestPlayerToV2PlayerResponseData(t *testing.T) {
	t.Parallel()

	t.Run("success with player", func(t *testing.T) {
		t.Parallel()

		const UUID = "01234567-89ab-cdef-0123-456789abcdef"
		now := time.Now()
		player := domaintest.NewPlayerBuilder(UUID, now).WithExperience(1000).BuildPtr()

		data, err := PlayerToV2PlayerResponseData(player)
		require.NoError(t, err)

		var response V2PlayerResponse
		err = json.Unmarshal(data, &response)
		require.NoError(t, err)

		require.True(t, response.Success)
		require.NotNil(t, response.Player)
		require.Nil(t, response.Cause)
		require.Equal(t, UUID, response.Player.UUID)
		require.Equal(t, 1000.0, response.Player.Experience)
	})

	t.Run("nil player", func(t *testing.T) {
		t.Parallel()

		data, err := PlayerToV2PlayerResponseData(nil)
		require.NoError(t, err)

		var response V2PlayerResponse
		err = json.Unmarshal(data, &response)
		require.NoError(t, err)

		require.True(t, response.Success)
		require.Nil(t, response.Player)
		require.Nil(t, response.Cause)
	})
}

func TestPlayerToV2PlayerErrorResponseData(t *testing.T) {
	t.Parallel()

	t.Run("error response", func(t *testing.T) {
		t.Parallel()

		cause := "Something went wrong"
		data, err := PlayerToV2PlayerErrorResponseData(cause)
		require.NoError(t, err)

		var response V2PlayerResponse
		err = json.Unmarshal(data, &response)
		require.NoError(t, err)

		require.False(t, response.Success)
		require.Nil(t, response.Player)
		require.NotNil(t, response.Cause)
		require.Equal(t, cause, *response.Cause)
	})
}