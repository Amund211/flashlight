package domain

import "errors"

var (
	ErrPlayerNotFound         = errors.New("player not found")
	ErrTemporarilyUnavailable = errors.New("temporarily unavailable")
	ErrUsernameNotFound       = errors.New("username not found")
	ErrNoSessions             = errors.New("no sessions found")
)
