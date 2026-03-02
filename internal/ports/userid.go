package ports

import (
	"fmt"
	"net/http"
)

func GetUserID(r *http.Request) string {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		return "<missing>"
	}

	if len(userID) < 20 {
		return "<short-user-id>"
	}

	return fmt.Sprintf("%.50s", userID)
}
