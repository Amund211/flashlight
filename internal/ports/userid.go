package ports

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

func GetUserID(r *http.Request) string {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		return "<missing>"
	}
	return fmt.Sprintf("%.50s", userID)
}

// HashUserID takes a user ID string and returns the SHA256 hash encoded as a hex string
func HashUserID(userID string) string {
	hash := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(hash[:])
}
