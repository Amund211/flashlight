package server

import (
	"net/http"

	"github.com/Amund211/flashlight/internal/ratelimiting"
	e "github.com/Amund211/flashlight/internal/errors"
)

func RateLimitMiddleware(rateLimiter ratelimiting.RateLimiter, next Handler) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rateLimiter.Allow(r.RemoteAddr) {
			writeErrorResponse(w, e.RatelimitExceededError)
			return
		}

		next(w, r)
	}
}
