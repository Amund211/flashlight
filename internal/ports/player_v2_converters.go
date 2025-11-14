package ports

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
)

// Player V2 response structures that closely match the domain structs with json tags

type PlayerResponseV2 struct {
	Success bool     `json:"success"`
	Player  *PlayerV2 `json:"player"`
	Cause   *string  `json:"cause,omitempty"`
}

type PlayerV2 struct {
	QueriedAt time.Time `json:"queriedAt"`
	UUID      string    `json:"uuid"`
	
	Displayname *string    `json:"displayname,omitempty"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
	LastLogout  *time.Time `json:"lastLogout,omitempty"`
	
	MissingBedwarsStats bool  `json:"missingBedwarsStats"`
	Experience          int64 `json:"experience"`
	
	Overall GamemodeStatsV2 `json:"overall"`
}

type GamemodeStatsV2 struct {
	Winstreak   *int `json:"winstreak,omitempty"`
	GamesPlayed int  `json:"gamesPlayed"`
	Wins        int  `json:"wins"`
	Losses      int  `json:"losses"`
	BedsBroken  int  `json:"bedsBroken"`
	BedsLost    int  `json:"bedsLost"`
	FinalKills  int  `json:"finalKills"`
	FinalDeaths int  `json:"finalDeaths"`
	Kills       int  `json:"kills"`
	Deaths      int  `json:"deaths"`
}

func domainGamemodeStatsToV2(stats *domain.GamemodeStatsPIT) GamemodeStatsV2 {
	return GamemodeStatsV2{
		Winstreak:   stats.Winstreak,
		GamesPlayed: stats.GamesPlayed,
		Wins:        stats.Wins,
		Losses:      stats.Losses,
		BedsBroken:  stats.BedsBroken,
		BedsLost:    stats.BedsLost,
		FinalKills:  stats.FinalKills,
		FinalDeaths: stats.FinalDeaths,
		Kills:       stats.Kills,
		Deaths:      stats.Deaths,
	}
}

func domainPlayerToPlayerV2(player *domain.PlayerPIT) *PlayerV2 {
	if player == nil {
		return nil
	}
	
	return &PlayerV2{
		QueriedAt:           player.QueriedAt,
		UUID:                player.UUID,
		Displayname:         player.Displayname,
		LastLogin:           player.LastLogin,
		LastLogout:          player.LastLogout,
		MissingBedwarsStats: player.MissingBedwarsStats,
		Experience:          player.Experience,
		Overall:             domainGamemodeStatsToV2(&player.Overall),
	}
}

func PlayerToPlayerResponseDataV2(player *domain.PlayerPIT) ([]byte, error) {
	response := PlayerResponseV2{
		Success: true,
	}
	
	// Always set the Player field, even if it's nil - this will ensure "player":null in JSON
	response.Player = domainPlayerToPlayerV2(player)
	
	data, err := json.Marshal(response)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal player V2 response: %w", err)
	}
	
	return data, nil
}

func PlayerToPlayerErrorResponseDataV2(cause string) ([]byte, error) {
	response := PlayerResponseV2{
		Success: false,
		Cause:   &cause,
	}
	
	data, err := json.Marshal(response)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal player V2 error response: %w", err)
	}
	
	return data, nil
}