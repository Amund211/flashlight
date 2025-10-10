package ports

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
)

// V2 Player response structures that closely match the domain structs with json tags

type V2PlayerResponse struct {
	Success bool       `json:"success"`
	Player  *V2Player  `json:"player"`
	Cause   *string    `json:"cause,omitempty"`
}

type V2Player struct {
	QueriedAt time.Time `json:"queriedAt"`
	UUID      string    `json:"uuid"`
	
	Displayname *string    `json:"displayname,omitempty"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
	LastLogout  *time.Time `json:"lastLogout,omitempty"`
	
	MissingBedwarsStats bool    `json:"missingBedwarsStats"`
	Experience          float64 `json:"experience"`
	
	Solo    V2GamemodeStats `json:"solo"`
	Doubles V2GamemodeStats `json:"doubles"`
	Threes  V2GamemodeStats `json:"threes"`
	Fours   V2GamemodeStats `json:"fours"`
	Overall V2GamemodeStats `json:"overall"`
}

type V2GamemodeStats struct {
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

func domainGamemodeStatsToV2(stats *domain.GamemodeStatsPIT) V2GamemodeStats {
	return V2GamemodeStats{
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

func domainPlayerToV2Player(player *domain.PlayerPIT) *V2Player {
	if player == nil {
		return nil
	}
	
	return &V2Player{
		QueriedAt:           player.QueriedAt,
		UUID:                player.UUID,
		Displayname:         player.Displayname,
		LastLogin:           player.LastLogin,
		LastLogout:          player.LastLogout,
		MissingBedwarsStats: player.MissingBedwarsStats,
		Experience:          player.Experience,
		Solo:                domainGamemodeStatsToV2(&player.Solo),
		Doubles:             domainGamemodeStatsToV2(&player.Doubles),
		Threes:              domainGamemodeStatsToV2(&player.Threes),
		Fours:               domainGamemodeStatsToV2(&player.Fours),
		Overall:             domainGamemodeStatsToV2(&player.Overall),
	}
}

func PlayerToV2PlayerResponseData(player *domain.PlayerPIT) ([]byte, error) {
	response := V2PlayerResponse{
		Success: true,
	}
	
	// Always set the Player field, even if it's nil - this will ensure "player":null in JSON
	response.Player = domainPlayerToV2Player(player)
	
	data, err := json.Marshal(response)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal V2 player response: %w", err)
	}
	
	return data, nil
}

func PlayerToV2PlayerErrorResponseData(cause string) ([]byte, error) {
	response := V2PlayerResponse{
		Success: false,
		Cause:   &cause,
	}
	
	data, err := json.Marshal(response)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal V2 player error response: %w", err)
	}
	
	return data, nil
}