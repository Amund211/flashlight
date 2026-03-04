package ports

import "net/http"

func GetUserAgent(r *http.Request) string {
	userAgent := r.UserAgent()
	if userAgent == "" {
		return "<missing>"
	}
	return userAgent
}
