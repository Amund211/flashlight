package domain

type Session struct {
	Start       PlayerPIT
	End         PlayerPIT
	Consecutive bool
}

type SessionDetail struct {
	Stats       []PlayerPIT
	Consecutive bool
}
