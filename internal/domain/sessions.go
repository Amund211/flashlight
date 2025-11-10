package domain

import (
	"time"
)

type Session struct {
	Start       PlayerPIT
	End         PlayerPIT
	Consecutive bool
}

// Playtime returns the duration of the session
func (s Session) Playtime() time.Duration {
	return s.End.QueriedAt.Sub(s.Start.QueriedAt)
}

// FinalKills returns the number of final kills gained during the session
func (s Session) FinalKills() int {
	return s.End.Overall.FinalKills - s.Start.Overall.FinalKills
}

// Wins returns the number of wins gained during the session
func (s Session) Wins() int {
	return s.End.Overall.Wins - s.Start.Overall.Wins
}

// FKDR returns the final kill/death ratio for the session
func (s Session) FKDR() float64 {
	finalKills := s.End.Overall.FinalKills - s.Start.Overall.FinalKills
	finalDeaths := s.End.Overall.FinalDeaths - s.Start.Overall.FinalDeaths
	if finalDeaths > 0 {
		return float64(finalKills) / float64(finalDeaths)
	} else if finalKills > 0 {
		return float64(finalKills)
	}
	return 0
}

// Stars returns the stars gained during the session
func (s Session) Stars() float64 {
	return s.End.Stars() - s.Start.Stars()
}

// BestSessions holds the best session for each metric
type BestSessions struct {
	Playtime   Session
	FinalKills Session
	Wins       Session
	FKDR       Session
	Stars      Session
}
