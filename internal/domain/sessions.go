package domain

type Session struct {
	Start       PlayerPIT
	End         PlayerPIT
	Consecutive bool
}

// BestSessions holds the best session for each metric
type BestSessions struct {
	Playtime   *Session
	FinalKills *Session
	Wins       *Session
	FKDR       *Session
	Stars      *Session
}
