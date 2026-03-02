package ports

import (
	"fmt"
	"net/http"
)

// UserID represents a user identifier extracted from an HTTP request.
type UserID string

// String returns the full user ID string.
func (u UserID) String() string {
	return string(u)
}

// LowCardinalityString returns a low cardinality representation of the user ID.
// For IDs shorter than 20 characters, it returns "<short-user-id>".
// Otherwise, it returns the full user ID.
func (u UserID) LowCardinalityString() string {
	if len(u) < 20 {
		return "<short-user-id>"
	}
	return string(u)
}

func GetUserID(r *http.Request) UserID {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		return UserID("<missing>")
	}
	return UserID(fmt.Sprintf("%.50s", userID))
}
